package editor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	guuid "github.com/google/uuid"
)

// RecordIndex maps entity IDs to their full JSON records.
type RecordIndex map[string]map[string]any

// GroupedRecords maps a grouping key to a list of records.
type GroupedRecords map[string][]map[string]any

// CascadeContext holds lookup data needed for cascade operations.
type CascadeContext struct {
	PlayableCharacters RecordIndex
	PlayableCostumes   RecordIndex
	Costumes           RecordIndex
	WeaponSkillDefs    GroupedRecords
	WeaponAbilityDefs  GroupedRecords
}

// NewCascadeContext loads the record indexes from Engels output.
func NewCascadeContext(outputDir string) *CascadeContext {
	ctx := &CascadeContext{}
	ctx.PlayableCharacters = loadRecordIndex(outputDir, "playable_characters.json")
	ctx.PlayableCostumes = loadRecordIndex(outputDir, "playable_costumes.json")
	ctx.Costumes = loadRecordIndex(outputDir, "costumes.json")
	ctx.WeaponSkillDefs = loadGroupedRecords(outputDir, "weapon_skills.json", "weapon_id")
	ctx.WeaponAbilityDefs = loadGroupedRecords(outputDir, "weapon_abilities.json", "weapon_id")
	log.Printf("[cascade] loaded: %d characters, %d costumes, %d weapon skill groups, %d weapon ability groups",
		len(ctx.PlayableCharacters), len(ctx.PlayableCostumes),
		len(ctx.WeaponSkillDefs), len(ctx.WeaponAbilityDefs))
	return ctx
}

// UpsertCharacterBundle inserts a character with default costume and weapon.
func (e *Editor) UpsertCharacterBundle(ctx *CascadeContext, row map[string]any) error {
	row = applyTableDefaults(e.Schema, "user_characters", row)
	userID := strVal(row["user_id"])
	characterID := strVal(row["character_id"])
	if userID == "" || characterID == "" {
		return fmt.Errorf("user_characters requires user_id and character_id")
	}

	charRec := ctx.PlayableCharacters[characterID]
	costumeID := defaultCostumeIDForCharacter(ctx, charRec)
	weaponID := strVal(charRec["DefaultWeaponId"])
	if weaponID == "0" {
		weaponID = ""
	}

	acquiredAt := time.Now().UTC().UnixMilli()

	db, err := e.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsertWithTx(tx, e.Schema, "user_characters", row); err != nil {
		return err
	}

	costumeUUID := ""
	if costumeID != "" {
		costumeUUID, err = ensureOwnedCostume(tx, e.Schema, userID, costumeID, acquiredAt)
		if err != nil {
			return err
		}
		ensureCostumeActiveSkill(tx, userID, costumeUUID, acquiredAt)
	}

	if weaponID != "" {
		weaponUUID, err := ensureOwnedWeapon(tx, e.Schema, userID, weaponID, acquiredAt)
		if err != nil {
			return err
		}
		ensureWeaponSupportRows(tx, ctx, userID, weaponID, weaponUUID, acquiredAt)
	}

	return tx.Commit()
}

// UpsertCostumeBundle inserts a costume with character and weapon scaffolding.
func (e *Editor) UpsertCostumeBundle(ctx *CascadeContext, row map[string]any) error {
	row = applyTableDefaults(e.Schema, "user_costumes", row)
	userID := strVal(row["user_id"])
	costumeID := strVal(row["costume_id"])
	if userID == "" || costumeID == "" {
		return fmt.Errorf("user_costumes requires user_id and costume_id")
	}

	costumeRec := ctx.PlayableCostumes[costumeID]
	characterID := strVal(costumeRec["CharacterId"])
	charRec := map[string]any{}
	if characterID != "" && characterID != "0" {
		charRec = ctx.PlayableCharacters[characterID]
	}
	weaponID := strVal(charRec["DefaultWeaponId"])
	if weaponID == "0" {
		weaponID = ""
	}
	userCostumeUUID := strVal(row["user_costume_uuid"])
	acquiredAt := toInt64(row["acquisition_datetime"])
	if acquiredAt <= 0 {
		acquiredAt = time.Now().UTC().UnixMilli()
	}

	db, err := e.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsertWithTx(tx, e.Schema, "user_costumes", row); err != nil {
		return err
	}
	ensureCostumeActiveSkill(tx, userID, userCostumeUUID, acquiredAt)

	if characterID != "" && characterID != "0" {
		charRow := applyTableDefaults(e.Schema, "user_characters", map[string]any{
			"user_id":      userID,
			"character_id": characterID,
		})
		upsertWithTx(tx, e.Schema, "user_characters", charRow)
	}

	if weaponID != "" {
		weaponUUID, err := ensureOwnedWeapon(tx, e.Schema, userID, weaponID, acquiredAt)
		if err == nil {
			ensureWeaponSupportRows(tx, ctx, userID, weaponID, weaponUUID, acquiredAt)
		}
	}

	return tx.Commit()
}

// DeleteRowCascade deletes a row with cascade cleanup.
func (e *Editor) DeleteRowCascade(ctx *CascadeContext, table string, key map[string]any) error {
	ts, ok := e.Schema[table]
	if !ok {
		return fmt.Errorf("table not found: %s", table)
	}
	if len(ts.PrimaryKey) == 0 {
		return fmt.Errorf("table has no primary key")
	}

	db, err := e.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch table {
	case "user_characters":
		deleteCharacterBundle(tx, ctx, strVal(key["user_id"]), strVal(key["character_id"]))
	case "user_costumes":
		userID := strVal(key["user_id"])
		ucUUID := strVal(key["user_costume_uuid"])
		// Find costume_id before deleting
		var costumeID string
		tx.QueryRow(`SELECT costume_id FROM user_costumes WHERE user_id = ? AND user_costume_uuid = ?`,
			userID, ucUUID).Scan(&costumeID)
		characterID := characterIDForCostume(ctx, costumeID)
		deleteCostumeBundle(tx, userID, ucUUID)
		deleteCharacterIfNoCostumes(tx, ctx, userID, characterID)
	default:
		deleteSimpleRow(tx, table, ts, key)
	}

	return tx.Commit()
}

// --- Internal cascade helpers ---

func ensureOwnedCostume(tx *sql.Tx, schema map[string]*TableSchema, userID, costumeID string, acquiredAt int64) (string, error) {
	var existing string
	err := tx.QueryRow(`SELECT user_costume_uuid FROM user_costumes WHERE user_id = ? AND costume_id = ? LIMIT 1`,
		userID, costumeID).Scan(&existing)
	if err == nil {
		return existing, nil
	}

	row := applyTableDefaults(schema, "user_costumes", map[string]any{
		"user_id":               userID,
		"user_costume_uuid":     scopedUUID(userID),
		"costume_id":            costumeID,
		"acquisition_datetime":  acquiredAt,
	})
	if err := upsertWithTx(tx, schema, "user_costumes", row); err != nil {
		return "", err
	}
	return strVal(row["user_costume_uuid"]), nil
}

func ensureCostumeActiveSkill(tx *sql.Tx, userID, ucUUID string, acquiredAt int64) {
	if ucUUID == "" {
		return
	}
	var exists int
	tx.QueryRow(`SELECT 1 FROM user_costume_active_skills WHERE user_id = ? AND user_costume_uuid = ? LIMIT 1`,
		userID, ucUUID).Scan(&exists)
	if exists > 0 {
		return
	}
	tx.Exec(`INSERT INTO user_costume_active_skills (user_id, user_costume_uuid, level, acquisition_datetime, latest_version) VALUES (?, ?, ?, ?, ?)`,
		userID, ucUUID, 1, acquiredAt, 0)
}

func ensureOwnedWeapon(tx *sql.Tx, schema map[string]*TableSchema, userID, weaponID string, acquiredAt int64) (string, error) {
	var existing string
	err := tx.QueryRow(`SELECT user_weapon_uuid FROM user_weapons WHERE user_id = ? AND weapon_id = ? LIMIT 1`,
		userID, weaponID).Scan(&existing)
	if err == nil {
		return existing, nil
	}

	row := applyTableDefaults(schema, "user_weapons", map[string]any{
		"user_id":              userID,
		"user_weapon_uuid":     scopedUUID(userID),
		"weapon_id":            weaponID,
		"acquisition_datetime": acquiredAt,
	})
	if err := upsertWithTx(tx, schema, "user_weapons", row); err != nil {
		return "", err
	}
	return strVal(row["user_weapon_uuid"]), nil
}

func ensureWeaponSupportRows(tx *sql.Tx, ctx *CascadeContext, userID, weaponID, uwUUID string, acquiredAt int64) {
	if uwUUID == "" {
		return
	}

	// Skills
	existingSkills := querySlotNumbers(tx, "user_weapon_skills", userID, uwUUID)
	for _, rec := range ctx.WeaponSkillDefs[weaponID] {
		slot := toIntVal(rec["slot_number"])
		if slot <= 0 || existingSkills[slot] {
			continue
		}
		tx.Exec(`INSERT INTO user_weapon_skills (user_id, user_weapon_uuid, slot_number, level) VALUES (?, ?, ?, ?)`,
			userID, uwUUID, slot, 1)
	}

	// Abilities
	existingAbils := querySlotNumbers(tx, "user_weapon_abilities", userID, uwUUID)
	for _, rec := range ctx.WeaponAbilityDefs[weaponID] {
		slot := toIntVal(rec["slot_number"])
		if slot <= 0 || existingAbils[slot] {
			continue
		}
		tx.Exec(`INSERT INTO user_weapon_abilities (user_id, user_weapon_uuid, slot_number, level) VALUES (?, ?, ?, ?)`,
			userID, uwUUID, slot, 1)
	}

	// Weapon note
	var noteExists int
	tx.QueryRow(`SELECT 1 FROM user_weapon_notes WHERE user_id = ? AND weapon_id = ? LIMIT 1`,
		userID, weaponID).Scan(&noteExists)
	if noteExists == 0 {
		tx.Exec(`INSERT INTO user_weapon_notes (user_id, weapon_id, max_level, max_limit_break_count, first_acquisition_datetime, latest_version) VALUES (?, ?, ?, ?, ?, ?)`,
			userID, weaponID, 1, 0, acquiredAt, acquiredAt)
	}

	// Weapon story
	var storyExists int
	tx.QueryRow(`SELECT 1 FROM user_weapon_stories WHERE user_id = ? AND weapon_id = ? LIMIT 1`,
		userID, weaponID).Scan(&storyExists)
	if storyExists == 0 {
		tx.Exec(`INSERT INTO user_weapon_stories (user_id, weapon_id, released_max_story_index, latest_version) VALUES (?, ?, ?, ?)`,
			userID, weaponID, 1, acquiredAt)
	}
}

func deleteCharacterBundle(tx *sql.Tx, ctx *CascadeContext, userID, characterID string) {
	if userID == "" || characterID == "" {
		return
	}
	rows, _ := tx.Query(`SELECT user_costume_uuid, costume_id FROM user_costumes WHERE user_id = ?`, userID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var ucUUID, costumeID string
			rows.Scan(&ucUUID, &costumeID)
			if characterIDForCostume(ctx, costumeID) != characterID {
				continue
			}
			deleteCostumeBundle(tx, userID, ucUUID)
		}
	}
	tx.Exec(`DELETE FROM user_characters WHERE user_id = ? AND character_id = ?`, userID, characterID)
}

func deleteCostumeBundle(tx *sql.Tx, userID, ucUUID string) {
	if ucUUID == "" {
		return
	}
	// Clean up deck references
	rows, _ := tx.Query(`SELECT user_deck_character_uuid FROM user_deck_characters WHERE user_id = ? AND user_costume_uuid = ?`,
		userID, ucUUID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var dcUUID string
			rows.Scan(&dcUUID)
			deleteDeckCharacterBundle(tx, userID, dcUUID)
		}
	}
	tx.Exec(`DELETE FROM user_costume_active_skills WHERE user_id = ? AND user_costume_uuid = ?`, userID, ucUUID)
	tx.Exec(`DELETE FROM user_costumes WHERE user_id = ? AND user_costume_uuid = ?`, userID, ucUUID)
}

func deleteDeckCharacterBundle(tx *sql.Tx, userID, dcUUID string) {
	if dcUUID == "" {
		return
	}
	tx.Exec(`DELETE FROM user_deck_sub_weapons WHERE user_id = ? AND user_deck_character_uuid = ?`, userID, dcUUID)
	tx.Exec(`DELETE FROM user_deck_parts WHERE user_id = ? AND user_deck_character_uuid = ?`, userID, dcUUID)
	for _, col := range []string{"user_deck_character_uuid01", "user_deck_character_uuid02", "user_deck_character_uuid03"} {
		tx.Exec(fmt.Sprintf(`UPDATE user_decks SET %s = NULL WHERE user_id = ? AND %s = ?`, col, col), userID, dcUUID)
	}
	tx.Exec(`DELETE FROM user_deck_characters WHERE user_id = ? AND user_deck_character_uuid = ?`, userID, dcUUID)
}

func deleteCharacterIfNoCostumes(tx *sql.Tx, ctx *CascadeContext, userID, characterID string) {
	if userID == "" || characterID == "" {
		return
	}
	rows, _ := tx.Query(`SELECT costume_id FROM user_costumes WHERE user_id = ?`, userID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cid string
			rows.Scan(&cid)
			if characterIDForCostume(ctx, cid) == characterID {
				return // Still has costumes for this character
			}
		}
	}
	tx.Exec(`DELETE FROM user_characters WHERE user_id = ? AND character_id = ?`, userID, characterID)
}

func deleteSimpleRow(tx *sql.Tx, table string, ts *TableSchema, key map[string]any) {
	var conds []string
	var vals []any
	for _, pk := range ts.PrimaryKey {
		if v, ok := key[pk]; ok {
			conds = append(conds, fmt.Sprintf("%s = ?", pk))
			vals = append(vals, v)
		}
	}
	if len(conds) > 0 {
		tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s", table, strings.Join(conds, " AND ")), vals...)
	}
}

// --- Shared helpers ---

func upsertWithTx(tx *sql.Tx, schema map[string]*TableSchema, table string, row map[string]any) error {
	ts, ok := schema[table]
	if !ok {
		return fmt.Errorf("table not found: %s", table)
	}
	var cols, phs []string
	var vals []any
	for _, col := range ts.Columns {
		if v, ok := row[col.Name]; ok {
			cols = append(cols, col.Name)
			phs = append(phs, "?")
			vals = append(vals, v)
		}
	}
	if len(cols) == 0 {
		return fmt.Errorf("no columns")
	}

	conflictSQL := ""
	if len(ts.PrimaryKey) > 0 {
		updateCols := []string{}
		for _, col := range cols {
			isPK := false
			for _, pk := range ts.PrimaryKey {
				if col == pk {
					isPK = true
					break
				}
			}
			if !isPK {
				updateCols = append(updateCols, fmt.Sprintf("%s = excluded.%s", col, col))
			}
		}
		if len(updateCols) > 0 {
			conflictSQL = fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s",
				strings.Join(ts.PrimaryKey, ", "), strings.Join(updateCols, ", "))
		} else {
			conflictSQL = fmt.Sprintf("ON CONFLICT (%s) DO NOTHING", strings.Join(ts.PrimaryKey, ", "))
		}
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) %s",
		table, strings.Join(cols, ", "), strings.Join(phs, ", "), conflictSQL)
	_, err := tx.Exec(query, vals...)
	return err
}

func applyTableDefaults(schema map[string]*TableSchema, table string, row map[string]any) map[string]any {
	// Auto-generate UUIDs for primary key UUID columns
	ts := schema[table]
	userID := strVal(row["user_id"])
	if ts != nil && userID != "" {
		for _, pk := range ts.PrimaryKey {
			if strings.HasSuffix(pk, "_uuid") && strVal(row[pk]) == "" {
				row[pk] = scopedUUID(userID)
			}
		}
	}

	now := time.Now().UTC().UnixMilli()
	switch table {
	case "user_costumes":
		setDefault(row, "limit_break_count", 0)
		setDefault(row, "level", 1)
		setDefault(row, "exp", 0)
		setDefault(row, "headup_display_view_id", 1)
		setDefault(row, "acquisition_datetime", now)
		setDefault(row, "awaken_count", 0)
		setDefault(row, "latest_version", 0)
	case "user_weapons":
		setDefault(row, "level", 1)
		setDefault(row, "exp", 0)
		setDefault(row, "limit_break_count", 0)
		setDefault(row, "is_protected", 0)
		setDefault(row, "acquisition_datetime", now)
		setDefault(row, "latest_version", 0)
	case "user_characters":
		setDefault(row, "level", 1)
		setDefault(row, "exp", 0)
		setDefault(row, "latest_version", 0)
	case "user_companions":
		setDefault(row, "headup_display_view_id", 1)
		setDefault(row, "level", 1)
		setDefault(row, "acquisition_datetime", now)
		setDefault(row, "latest_version", 0)
	}
	return row
}

func setDefault(row map[string]any, key string, val any) {
	if v, ok := row[key]; !ok || strVal(v) == "" {
		row[key] = val
	}
}

func defaultCostumeIDForCharacter(ctx *CascadeContext, charRec map[string]any) string {
	if charRec == nil {
		return ""
	}
	costumeID := strVal(charRec["DefaultCostumeId"])
	if costumeID != "" && costumeID != "0" {
		return costumeID
	}
	characterID := strVal(charRec["id"])
	if characterID == "" {
		return ""
	}
	// Find first costume for this character
	type sortable struct {
		id    string
		intID int
	}
	var candidates []sortable
	for id, rec := range ctx.PlayableCostumes {
		if strVal(rec["CharacterId"]) == characterID {
			n, _ := strconv.Atoi(id)
			candidates = append(candidates, sortable{id, n})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].intID < candidates[j].intID
	})
	return candidates[0].id
}

func characterIDForCostume(ctx *CascadeContext, costumeID string) string {
	if rec, ok := ctx.PlayableCostumes[costumeID]; ok {
		return strVal(rec["CharacterId"])
	}
	if rec, ok := ctx.Costumes[costumeID]; ok {
		return strVal(rec["CharacterId"])
	}
	return ""
}

func querySlotNumbers(tx *sql.Tx, table, userID, uwUUID string) map[int]bool {
	result := map[int]bool{}
	rows, err := tx.Query(fmt.Sprintf("SELECT slot_number FROM %s WHERE user_id = ? AND user_weapon_uuid = ?", table),
		userID, uwUUID)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var slot int
		rows.Scan(&slot)
		result[slot] = true
	}
	return result
}

func scopedUUID(userID string) string {
	return guuid.New().String()
}

func strVal(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int64:
		return val
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	}
	return 0
}

func toIntVal(v any) int {
	return int(toInt64(v))
}

// --- Record loading ---

func loadRecordIndex(outputDir, fileName string) RecordIndex {
	path := filepath.Join(outputDir, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return RecordIndex{}
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return RecordIndex{}
	}
	result := make(RecordIndex, len(payload.Records))
	for _, rec := range payload.Records {
		id := strVal(rec["id"])
		if id != "" {
			result[id] = rec
		}
	}
	return result
}

func loadGroupedRecords(outputDir, fileName, keyField string) GroupedRecords {
	path := filepath.Join(outputDir, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return GroupedRecords{}
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return GroupedRecords{}
	}
	result := make(GroupedRecords)
	for _, rec := range payload.Records {
		key := strVal(rec[keyField])
		if key != "" {
			result[key] = append(result[key], rec)
		}
	}
	return result
}
