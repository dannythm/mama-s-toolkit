package editor

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ImageIndex maps category → assetID → set of filenames.
type ImageIndex map[string]map[string]map[string]bool

// DeckViewer handles active deck rendering with image resolution.
type DeckViewer struct {
	editor     *Editor
	cascade    *CascadeContext
	imageIndex ImageIndex
	imagesDir  string
}

// NewDeckViewer creates a deck viewer with image index.
func NewDeckViewer(editor *Editor, cascade *CascadeContext, imagesDir string) *DeckViewer {
	dv := &DeckViewer{
		editor:    editor,
		cascade:   cascade,
		imagesDir: imagesDir,
	}
	dv.imageIndex = dv.buildImageIndex()
	return dv
}

func (dv *DeckViewer) buildImageIndex() ImageIndex {
	index := ImageIndex{}
	for _, category := range []string{"costume", "weapon", "companions"} {
		root := filepath.Join(dv.imagesDir, category)
		catIndex := map[string]map[string]bool{}
		entries, err := os.ReadDir(root)
		if err != nil {
			index[category] = catIndex
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			files := map[string]bool{}
			children, _ := os.ReadDir(filepath.Join(root, entry.Name()))
			for _, child := range children {
				if !child.IsDir() {
					files[child.Name()] = true
				}
			}
			catIndex[entry.Name()] = files
		}
		index[category] = catIndex
	}
	return index
}

// FindImageURL resolves an entity to its image URL.
func (dv *DeckViewer) FindImageURL(category string, assetIDs []string, variants []string) string {
	catIndex := dv.imageIndex[category]
	if catIndex == nil {
		return ""
	}
	for _, assetID := range assetIDs {
		if assetID == "" {
			continue
		}
		files := catIndex[assetID]
		if files == nil {
			continue
		}
		for _, variant := range variants {
			for _, ext := range []string{".png", ".jpg", ".webp"} {
				name := variant + ext
				if files[name] {
					return fmt.Sprintf("/images/%s/%s/%s", category, assetID, name)
				}
			}
		}
		// Return first available file
		for name := range files {
			return fmt.Sprintf("/images/%s/%s/%s", category, assetID, name)
		}
	}
	return ""
}

// ActiveDeck returns the selected deck with slot details.
func (dv *DeckViewer) ActiveDeck(userID, deckType, deckNumber string) (map[string]any, error) {
	db, err := dv.editor.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Get all decks for user
	rows, err := db.Query(`SELECT * FROM user_decks WHERE user_id = ? ORDER BY deck_type, user_deck_number, ROWID`, userID)
	if err != nil {
		return map[string]any{"decks": []any{}, "selectedDeckKey": "", "deck": nil, "slots": []any{}}, nil
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var deckRows []map[string]any
	for rows.Next() {
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = normalizeValue(vals[i])
		}
		deckRows = append(deckRows, row)
	}

	if len(deckRows) == 0 {
		return map[string]any{"decks": []any{}, "selectedDeckKey": "", "deck": nil, "slots": []any{}}, nil
	}

	// Build deck options
	var selectedDeck map[string]any
	selectedType := -1
	selectedNum := -1
	if strings.TrimSpace(deckType) != "" {
		selectedType, _ = strconv.Atoi(deckType)
	}
	if strings.TrimSpace(deckNumber) != "" {
		selectedNum, _ = strconv.Atoi(deckNumber)
	}

	var deckOptions []map[string]any
	for i, row := range deckRows {
		rowType := toIntAny(row["deck_type"])
		rowNum := toIntAny(row["user_deck_number"])
		key := fmt.Sprintf("%d:%d", rowType, rowNum)
		deckOptions = append(deckOptions, map[string]any{
			"key":          key,
			"displayIndex": i + 1,
			"deckType":     rowType,
			"deckNumber":   rowNum,
			"name":         fmt.Sprintf("Deck %d", i+1),
			"power":        toIntAny(row["power"]),
			"label":        fmt.Sprintf("Deck %d", i+1),
		})
		if selectedType == rowType && selectedNum == rowNum {
			selectedDeck = row
		}
	}

	deck := selectedDeck
	if deck == nil {
		deck = deckRows[0]
	}
	deckTypeVal := toIntAny(deck["deck_type"])
	deckNumVal := toIntAny(deck["user_deck_number"])
	selectedKey := fmt.Sprintf("%d:%d", deckTypeVal, deckNumVal)

	displayIndex := 1
	for _, opt := range deckOptions {
		if opt["key"] == selectedKey {
			displayIndex = opt["displayIndex"].(int)
		}
	}

	// Resolve slots: slot order is 2→center, 1→left, 3→right
	slotColumns := []struct {
		slot int
		col  string
	}{
		{1, "user_deck_character_uuid02"},
		{2, "user_deck_character_uuid01"},
		{3, "user_deck_character_uuid03"},
	}

	var slots []map[string]any
	for _, sc := range slotColumns {
		dcUUID := strVal(deck[sc.col])
		if dcUUID == "" {
			slots = append(slots, map[string]any{"slot": sc.slot})
			continue
		}
		slot := dv.activeDeckSlot(db, userID, dcUUID, sc.slot)
		slots = append(slots, slot)
	}

	return map[string]any{
		"decks":           deckOptions,
		"selectedDeckKey": selectedKey,
		"deck": map[string]any{
			"displayIndex": displayIndex,
			"deckType":     deckTypeVal,
			"deckNumber":   deckNumVal,
			"name":         fmt.Sprintf("Deck %d", displayIndex),
			"power":        toIntAny(deck["power"]),
		},
		"slots": slots,
	}, nil
}

func (dv *DeckViewer) activeDeckSlot(db *sql.DB, userID, dcUUID string, slotNum int) map[string]any {
	// Get deck character row
	row := queryRowMap(db, `SELECT * FROM user_deck_characters WHERE user_id = ? AND user_deck_character_uuid = ? LIMIT 1`,
		userID, dcUUID)
	if row == nil {
		return map[string]any{"slot": slotNum}
	}

	costumeUUID := strVal(row["user_costume_uuid"])
	weaponUUID := strVal(row["main_user_weapon_uuid"])
	companionUUID := strVal(row["user_companion_uuid"])
	thoughtUUID := strVal(row["user_thought_uuid"])

	costumeRow := queryRowMap(db, `SELECT * FROM user_costumes WHERE user_id = ? AND user_costume_uuid = ? LIMIT 1`, userID, costumeUUID)
	weaponRow := queryRowMap(db, `SELECT * FROM user_weapons WHERE user_id = ? AND user_weapon_uuid = ? LIMIT 1`, userID, weaponUUID)
	companionRow := queryRowMap(db, `SELECT * FROM user_companions WHERE user_id = ? AND user_companion_uuid = ? LIMIT 1`, userID, companionUUID)
	thoughtRow := queryRowMap(db, `SELECT * FROM user_thoughts WHERE user_id = ? AND user_thought_uuid = ? LIMIT 1`, userID, thoughtUUID)

	costumeID := strVal(costumeRow["costume_id"])
	weaponID := strVal(weaponRow["weapon_id"])
	companionID := strVal(companionRow["companion_id"])
	thoughtID := strVal(thoughtRow["thought_id"])

	costumeRec := dv.cascade.Costumes[costumeID]
	if costumeRec == nil {
		costumeRec = dv.cascade.PlayableCostumes[costumeID]
	}
	weaponRec := RecordIndex(nil) // We'll use a simple lookup
	_ = weaponRec
	companionRec := map[string]any{}
	thoughtRec := map[string]any{}
	_ = companionRec
	_ = thoughtRec

	result := map[string]any{
		"slot":              slotNum,
		"deckCharacterUuid": dcUUID,
		"power":             toIntAny(row["power"]),
	}

	if costumeID != "" {
		cr := costumeRec
		if cr == nil {
			cr = map[string]any{}
		}
		result["costume"] = map[string]any{
			"userCostumeUuid": costumeUUID,
			"costumeId":       toIntAny(costumeID),
			"characterId":     toIntAny(cr["CharacterId"]),
			"name":            orStr(strVal(cr["name"]), fmt.Sprintf("Costume %s", costumeID)),
			"characterName":   strVal(cr["character_name"]),
			"imageUrl": dv.FindImageURL("costume",
				[]string{strVal(cr["costume_actor_asset_id"])},
				[]string{"gacha"}),
		}
	}

	if weaponID != "" {
		result["weapon"] = map[string]any{
			"userWeaponUuid": weaponUUID,
			"weaponId":       toIntAny(weaponID),
			"name":           fmt.Sprintf("Weapon %s", weaponID),
			"imageUrl":       "",
		}
	}

	if companionID != "" {
		result["companion"] = map[string]any{
			"userCompanionUuid": companionUUID,
			"companionId":       toIntAny(companionID),
			"name":              fmt.Sprintf("Companion %s", companionID),
			"imageUrl":          "",
		}
	}

	if thoughtID != "" {
		result["thought"] = map[string]any{
			"userThoughtUuid": thoughtUUID,
			"thoughtId":       toIntAny(thoughtID),
			"name":            fmt.Sprintf("Thought %s", thoughtID),
		}
	}

	// Sub-weapons
	subWeaponRows := queryRowsSlice(db,
		`SELECT sw.ordinal, sw.user_weapon_uuid, w.weapon_id FROM user_deck_sub_weapons sw
		 JOIN user_weapons w ON w.user_id = sw.user_id AND w.user_weapon_uuid = sw.user_weapon_uuid
		 WHERE sw.user_id = ? AND sw.user_deck_character_uuid = ? ORDER BY sw.ordinal`,
		userID, dcUUID)
	var subWeapons []map[string]any
	for _, sr := range subWeaponRows {
		subWeapons = append(subWeapons, map[string]any{
			"userWeaponUuid": strVal(sr["user_weapon_uuid"]),
			"weaponId":       toIntAny(sr["weapon_id"]),
			"name":           fmt.Sprintf("Weapon %s", strVal(sr["weapon_id"])),
		})
	}
	result["subWeapons"] = subWeapons

	// Parts
	partRows := queryRowsSlice(db,
		`SELECT dp.ordinal, dp.user_parts_uuid, p.parts_id FROM user_deck_parts dp
		 JOIN user_parts p ON p.user_id = dp.user_id AND p.user_parts_uuid = dp.user_parts_uuid
		 WHERE dp.user_id = ? AND dp.user_deck_character_uuid = ? ORDER BY dp.ordinal`,
		userID, dcUUID)
	var parts []map[string]any
	for _, pr := range partRows {
		parts = append(parts, map[string]any{
			"userPartsUuid": strVal(pr["user_parts_uuid"]),
			"partsId":       toIntAny(pr["parts_id"]),
			"name":          fmt.Sprintf("Parts %s", strVal(pr["parts_id"])),
		})
	}
	result["parts"] = parts

	return result
}

// --- Query helpers ---

func queryRowMap(db *sql.DB, query string, args ...any) map[string]any {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	if !rows.Next() {
		return nil
	}
	vals := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil
	}
	result := make(map[string]any, len(columns))
	for i, col := range columns {
		result[col] = normalizeValue(vals[i])
	}
	return result
}

func queryRowsSlice(db *sql.DB, query string, args ...any) []map[string]any {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = normalizeValue(vals[i])
		}
		result = append(result, row)
	}
	return result
}

func toIntAny(v any) int {
	switch val := v.(type) {
	case int64:
		return int(val)
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return 0
}

func orStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
