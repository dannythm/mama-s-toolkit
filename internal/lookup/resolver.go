// Package lookup resolves numeric entity IDs to human-readable labels
// using the Engels extraction output JSON files.
package lookup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Entry is a resolved label for an entity ID.
type Entry struct {
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
	Group  string `json:"group,omitempty"`
}

// OwnedEntityRef holds a resolved entity with slot-level detail.
type OwnedEntityRef struct {
	Entry           Entry
	EntityID        string
	LimitBreak      int
	AbilitySlots    map[int]Entry // slot_number → entry
	SkillSlots      map[int]Entry // slot_number → entry
	ActiveSkillByLB map[int]Entry // limit_break_threshold → entry
}

// columnFileMap maps DB column names to Engels output file names.
var columnFileMap = map[string]string{
	"material_id":                  "materials.json",
	"consumable_item_id":           "consumables.json",
	"weapon_id":                    "weapons.json",
	"character_id":                 "characters.json",
	"costume_id":                   "costumes.json",
	"companion_id":                 "companions.json",
	"thought_id":                   "thoughts.json",
	"parts_id":                     "parts.json",
	"ability_id":                   "abilities.json",
	"skill_id":                     "skills.json",
	"important_item_id":            "important_items.json",
	"premium_item_id":              "premium_items.json",
	"mission_id":                   "missions.json",
	"quest_id":                     "quests.json",
	"quest_mission_id":             "quest_missions.json",
	"tutorial_type":                "tutorials.json",
	"shop_id":                      "shops.json",
	"shop_item_id":                 "shop_items.json",
	"gacha_medal_id":               "gacha_medals.json",
	"gacha_id":                     "gacha_banners.json",
	"gift_text_id":                 "gift_texts.json",
	"character_board_id":           "character_boards.json",
	"character_board_ability_id":   "character_board_abilities.json",
	"character_board_status_up_id": "character_board_status_ups.json",
	"weapon_awaken_id":             "weapon_awakens.json",
	"costume_awaken_ability_id":    "costume_awaken_abilities.json",
	"main_quest_chapter_id":        "main_quests.json",
	"event_quest_chapter_id":       "event_quests.json",
	"extra_quest_id":               "extra_quests.json",
	"side_story_quest_id":          "side_story_quests.json",
	"cage_ornament_id":             "cage_ornament_rewards.json",
}

// optionFileOverrides specifies alternative files for dropdown options.
var optionFileOverrides = map[string]string{
	"character_id": "playable_characters.json",
	"costume_id":   "playable_costumes.json",
}

// columnAliases maps alternate column names to their canonical form.
var columnAliases = map[string]string{
	"favorite_costume_id":       "costume_id",
	"dressup_costume_id":        "costume_id",
	"description_gift_text_id":  "gift_text_id",
}

// Registry holds all lookup data loaded from Engels output.
type Registry struct {
	OutputDir        string
	Columns          map[string]map[string]Entry // column → id → entry
	OptionColumns    map[string]map[string]Entry // column → id → entry (for dropdowns)
	WeaponSkillSlots map[string]map[int]Entry    // weapon_id → slot → entry
	WeaponAbilSlots  map[string]map[int]Entry    // weapon_id → slot → entry
	CostumeActSkills map[string]map[int]Entry    // costume_id → lb_threshold → entry
	Summary          RegistrySummary
}

// RegistrySummary reports lookup availability.
type RegistrySummary struct {
	Enabled    bool     `json:"enabled"`
	SourcePath string   `json:"sourcePath"`
	EntryCount int      `json:"entryCount"`
	Kinds      []string `json:"kinds"`
}

// NewRegistry creates a lookup registry from the Engels output directory.
func NewRegistry(outputDir string) *Registry {
	r := &Registry{
		OutputDir:        outputDir,
		Columns:          make(map[string]map[string]Entry),
		OptionColumns:    make(map[string]map[string]Entry),
		WeaponSkillSlots: make(map[string]map[int]Entry),
		WeaponAbilSlots:  make(map[string]map[int]Entry),
		CostumeActSkills: make(map[string]map[int]Entry),
		Summary: RegistrySummary{
			SourcePath: outputDir,
		},
	}
	r.load()
	return r
}

func (r *Registry) load() {
	if _, err := os.Stat(r.OutputDir); os.IsNotExist(err) {
		log.Printf("[lookup] output dir %s not found, lookups disabled", r.OutputDir)
		return
	}

	for column, fileName := range columnFileMap {
		if column == "gacha_id" {
			r.Columns[column] = r.loadGachaLookup(fileName)
		} else {
			r.Columns[column] = r.loadLookupFile(fileName)
		}
	}

	for column, fileName := range optionFileOverrides {
		entries := r.loadLookupFile(fileName)
		if len(entries) > 0 {
			r.OptionColumns[column] = entries
		}
	}

	// Fill option columns from regular columns where no override exists
	for column, entries := range r.Columns {
		if _, ok := r.OptionColumns[column]; !ok {
			r.OptionColumns[column] = entries
		}
	}

	r.WeaponSkillSlots = r.loadSlotLookup("weapon_skills.json", "weapon_id", "")
	r.WeaponAbilSlots = r.loadSlotLookup("weapon_abilities.json", "weapon_id", "")
	r.CostumeActSkills = r.loadSlotLookup("costume_active_skills.json", "costume_id", "limit_break_count_lower_limit")

	var kinds []string
	total := 0
	for col, entries := range r.Columns {
		if len(entries) > 0 {
			kinds = append(kinds, col)
			total += len(entries)
		}
	}
	sort.Strings(kinds)
	r.Summary.Enabled = len(kinds) > 0
	r.Summary.Kinds = kinds
	r.Summary.EntryCount = total
	log.Printf("[lookup] loaded %d entries across %d kinds from %s", total, len(kinds), r.OutputDir)
}

// loadLookupFile loads a single Engels output JSON and builds id→Entry map.
func (r *Registry) loadLookupFile(fileName string) map[string]Entry {
	path := filepath.Join(r.OutputDir, fileName)
	records, err := loadRecords(path)
	if err != nil {
		return nil
	}
	entries := make(map[string]Entry, len(records))
	for _, rec := range records {
		id := stringify(rec["id"])
		if id == "" {
			continue
		}
		entries[id] = Entry{
			Label:  stringOr(rec, "name", id),
			Detail: detailFromRecord(rec),
			Group:  groupForRecord(fileName, rec),
		}
	}
	return entries
}

// loadSlotLookup loads slot-indexed lookups (weapon skills, abilities, costume active skills).
func (r *Registry) loadSlotLookup(fileName, ownerField, limitField string) map[string]map[int]Entry {
	path := filepath.Join(r.OutputDir, fileName)
	records, err := loadRecords(path)
	if err != nil {
		return nil
	}
	result := make(map[string]map[int]Entry)
	for _, rec := range records {
		ownerID := stringify(rec[ownerField])
		if ownerID == "" {
			continue
		}
		var slotKey int
		if limitField != "" {
			slotKey = toInt(rec[limitField])
		} else {
			slotKey = toInt(rec["slot_number"])
		}
		if result[ownerID] == nil {
			result[ownerID] = make(map[int]Entry)
		}
		result[ownerID][slotKey] = Entry{
			Label:  stringOr(rec, "name", ""),
			Detail: detailFromRecord(rec),
			Group:  groupForRecord(fileName, rec),
		}
	}
	return result
}

// loadGachaLookup groups gacha banners by DestinationDomainId, deduplicating step-ups.
func (r *Registry) loadGachaLookup(fileName string) map[string]Entry {
	path := filepath.Join(r.OutputDir, fileName)
	records, err := loadRecords(path)
	if err != nil {
		return nil
	}

	type bannerGroup struct {
		records []map[string]any
	}
	grouped := make(map[string]*bannerGroup)
	for _, rec := range records {
		destID := toInt(rec["DestinationDomainId"])
		if destID <= 0 {
			continue
		}
		assetName := stringify(rec["BannerAssetName"])
		var gachaID string
		if strings.HasPrefix(assetName, "step_up_") {
			gachaID = strconv.Itoa(destID / 1000)
		} else {
			gachaID = strconv.Itoa(destID)
		}
		if grouped[gachaID] == nil {
			grouped[gachaID] = &bannerGroup{}
		}
		grouped[gachaID].records = append(grouped[gachaID].records, rec)
	}

	result := make(map[string]Entry, len(grouped))
	for gachaID, g := range grouped {
		// Pick best record: prefer name_found, then highest SortOrderDesc
		sort.Slice(g.records, func(i, j int) bool {
			ai := g.records[i]
			aj := g.records[j]
			fi, fj := 0, 0
			if toBool(ai["name_found"]) {
				fi = 1
			}
			if toBool(aj["name_found"]) {
				fj = 1
			}
			if fi != fj {
				return fi > fj
			}
			si := toInt(ai["SortOrderDesc"])
			sj := toInt(aj["SortOrderDesc"])
			if si != sj {
				return si > sj
			}
			return toInt(ai["id"]) < toInt(aj["id"])
		})
		chosen := g.records[0]
		mode := "banner"
		if strings.HasPrefix(stringify(chosen["BannerAssetName"]), "step_up_") {
			mode = "step-up"
		}
		detail := joinDetail(mode, fmt.Sprintf("banner %v", chosen["id"]),
			conditionalStr(stringify(chosen["BannerAssetName"]) != "", "asset "+stringify(chosen["BannerAssetName"])),
			detailFromRecord(chosen))
		result[gachaID] = Entry{
			Label:  stringOr(chosen, "name", "Gacha "+gachaID),
			Detail: detail,
			Group:  groupForRecord(fileName, chosen),
		}
	}
	return result
}

// ResolveColumnEntries returns all entries for a column (for dropdown options).
func (r *Registry) ResolveColumnEntries(column string) map[string]Entry {
	canonical := resolveAlias(column)
	if entries, ok := r.OptionColumns[canonical]; ok {
		return entries
	}
	if entries, ok := r.OptionColumns[column]; ok {
		return entries
	}
	for suffix, entries := range r.OptionColumns {
		if strings.HasSuffix(column, suffix) {
			return entries
		}
	}
	return nil
}

// ResolveAnnotation resolves a single column+value to a label entry.
func (r *Registry) ResolveAnnotation(column string, value any) *Entry {
	id := strings.TrimSpace(stringify(value))
	if id == "" || id == "0" {
		return nil
	}
	canonical := resolveAlias(column)
	if entries, ok := r.Columns[canonical]; ok {
		if e, ok := entries[id]; ok {
			return &e
		}
	}
	if entries, ok := r.Columns[column]; ok {
		if e, ok := entries[id]; ok {
			return &e
		}
	}
	for suffix, entries := range r.Columns {
		if strings.HasSuffix(column, suffix) {
			if e, ok := entries[id]; ok {
				return &e
			}
		}
	}
	return nil
}

// CostumeActiveSkillForLB resolves the active skill at a given limit break.
func (r *Registry) CostumeActiveSkillForLB(costumeID string, limitBreak int) *Entry {
	byLimit, ok := r.CostumeActSkills[costumeID]
	if !ok {
		return nil
	}
	var bestEntry *Entry
	bestThreshold := -1
	for threshold, e := range byLimit {
		if threshold <= limitBreak && threshold > bestThreshold {
			eCopy := e
			bestEntry = &eCopy
			bestThreshold = threshold
		}
	}
	if bestEntry != nil {
		return bestEntry
	}
	if e, ok := byLimit[0]; ok {
		return &e
	}
	return nil
}

// --- Helpers ---

func resolveAlias(column string) string {
	if alias, ok := columnAliases[column]; ok {
		return alias
	}
	return column
}

func loadRecords(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return payload.Records, nil
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	}
	return 0
}

func toBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val == "true" || val == "1"
	}
	return false
}

func stringOr(rec map[string]any, key, fallback string) string {
	if v, ok := rec[key]; ok {
		s := stringify(v)
		if s != "" {
			return s
		}
	}
	return fallback
}

func conditionalStr(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}

var rarityPattern = regexp.MustCompile(`(?i)^(SS?R|[ABCR])\b`)

func detailFromRecord(rec map[string]any) string {
	parts := []string{}
	if rarity := stringify(rec["rarity"]); rarity != "" {
		parts = append(parts, rarity)
	} else if rarityType := stringify(rec["RarityType"]); rarityType != "" {
		parts = append(parts, "rarity "+rarityType)
	}
	if charName := stringify(rec["character_name"]); charName != "" {
		parts = append(parts, charName)
	}
	if attr := stringify(rec["attribute"]); attr != "" {
		parts = append(parts, attr)
	}
	return joinDetail(parts...)
}

func groupForRecord(fileName string, rec map[string]any) string {
	if g := stringify(rec["group"]); g != "" {
		return g
	}
	if charName := stringify(rec["character_name"]); charName != "" {
		return charName
	}
	return ""
}

func joinDetail(parts ...string) string {
	var filtered []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, " · ")
}
