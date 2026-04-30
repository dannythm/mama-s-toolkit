package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mama-toolkit/internal/lookup"
	"mama-toolkit/internal/patcher"
)

// handleOverview serves GET /api/overview
func handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if dbEditor == nil {
		writeJSON(w, map[string]any{
			"dbPath":      "",
			"tableCount":  0,
			"userCount":   0,
			"rowCounts":   map[string]int{},
			"schema":      map[string]any{},
			"tableGroups": []any{},
			"users":       []any{},
		})
		return
	}
	overview, err := dbEditor.Overview()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, overview)
}

// handleUserRoutes dispatches /api/user/{id}/... routes
func handleUserRoutes(w http.ResponseWriter, r *http.Request) {
	if dbEditor == nil {
		http.Error(w, "editor not available", http.StatusServiceUnavailable)
		return
	}

	// Parse: /api/user/{id} or /api/user/{id}/summary or /api/user/{id}/active-deck or /api/user/{id}/sessions
	path := strings.TrimPrefix(r.URL.Path, "/api/user/")
	parts := strings.SplitN(path, "/", 2)
	userID := parts[0]
	subRoute := ""
	if len(parts) > 1 {
		subRoute = parts[1]
	}

	if userID == "" {
		http.Error(w, "user id required", http.StatusBadRequest)
		return
	}

	switch {
	case subRoute == "summary" && r.Method == http.MethodGet:
		summary, err := dbEditor.UserSummary(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, summary)

	case subRoute == "active-deck" && r.Method == http.MethodGet:
		if deckViewer == nil {
			writeJSON(w, map[string]any{"decks": []any{}, "selectedDeckKey": "", "deck": nil, "slots": []any{}})
			return
		}
		deckType := r.URL.Query().Get("deck_type")
		deckNumber := r.URL.Query().Get("deck_number")
		result, err := deckViewer.ActiveDeck(userID, deckType, deckNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)

	case subRoute == "sessions" && r.Method == http.MethodDelete:
		deleted, err := dbEditor.InvalidateSessions(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"deletedSessions": deleted})

	case subRoute == "" && r.Method == http.MethodDelete:
		if err := dbEditor.DeleteUser(userID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleTableRoutes dispatches /api/table/{name} routes
func handleTableRoutes(w http.ResponseWriter, r *http.Request) {
	if dbEditor == nil {
		http.Error(w, "editor not available", http.StatusServiceUnavailable)
		return
	}

	table := strings.TrimPrefix(r.URL.Path, "/api/table/")
	if table == "" {
		http.Error(w, "table name required", http.StatusBadRequest)
		return
	}

	userID := r.URL.Query().Get("user_id")

	switch r.Method {
	case http.MethodGet:
		rows, err := dbEditor.TableRows(table, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Annotate rows with lookup labels
		var annotations []map[string]any
		for _, row := range rows {
			ann := make(map[string]any)
			for col, val := range row {
				entry := lookupReg.ResolveAnnotation(col, val)
				if entry != nil {
					ann[col] = entry
				}
			}
			annotations = append(annotations, ann)
		}

		writeJSON(w, map[string]any{
			"table":       table,
			"rows":        rows,
			"annotations": annotations,
			"schema":      dbEditor.Schema[table],
		})

	case http.MethodPost:
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Cascade logic for special tables
		var err error
		switch table {
		case "user_characters":
			err = dbEditor.UpsertCharacterBundle(cascadeCtx, body)
		case "user_costumes":
			err = dbEditor.UpsertCostumeBundle(cascadeCtx, body)
		default:
			err = dbEditor.UpsertRow(table, body)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	case http.MethodDelete:
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		err := dbEditor.DeleteRowCascade(cascadeCtx, table, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLookups serves GET /api/lookups/{column}
func handleLookups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	column := strings.TrimPrefix(r.URL.Path, "/api/lookups/")
	if column == "" {
		http.Error(w, "column name required", http.StatusBadRequest)
		return
	}

	entries := lookupReg.ResolveColumnEntries(column)
	if entries == nil {
		entries = map[string]lookup.Entry{}
	}

	// Convert to array format expected by frontend
	// Use a pre-allocated slice (not nil) so JSON serializes as [] not null
	type option struct {
		Value  string `json:"value"`
		Label  string `json:"label"`
		Detail string `json:"detail,omitempty"`
		Group  string `json:"group,omitempty"`
	}
	options := make([]option, 0, len(entries))
	for id, entry := range entries {
		options = append(options, option{
			Value:  id,
			Label:  entry.Label,
			Detail: entry.Detail,
			Group:  entry.Group,
		})
	}

	writeJSON(w, options)
}

// handleGachaBanners serves GET/POST /api/master-data/gacha-banners
func handleGachaBanners(w http.ResponseWriter, r *http.Request) {
	masterDataDir := filepath.Join(dataDir, "assets", "master_data")
	momBannerPath := filepath.Join(masterDataDir, "EntityMMomBannerTable.json")

	switch r.Method {
	case http.MethodGet:
		catalog, err := buildBannerCatalog(momBannerPath)
		if err != nil {
			log.Printf("[banners] catalog error: %v", err)
			writeJSON(w, map[string]any{"enabled": false})
			return
		}
		writeJSON(w, catalog)

	case http.MethodPost:
		var body struct {
			ActiveBannerIds []int `json:"activeBannerIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Save active banner IDs and rebuild catalog
		if err := saveActiveBannerIDs(momBannerPath, body.ActiveBannerIds); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		catalog, _ := buildBannerCatalog(momBannerPath)
		writeJSON(w, catalog)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEvents serves GET/POST /api/master-data/events
func handleEvents(w http.ResponseWriter, r *http.Request) {
	masterDataDir := filepath.Join(dataDir, "assets", "master_data")

	switch r.Method {
	case http.MethodGet:
		catalog, err := buildEventCatalog(masterDataDir)
		if err != nil {
			log.Printf("[events] catalog error: %v", err)
			writeJSON(w, map[string]any{"groups": map[string]any{}})
			return
		}
		writeJSON(w, catalog)

	case http.MethodPost:
		var body struct {
			Group     string `json:"group"`
			ActiveIds []int  `json:"activeIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if err := saveEventSelection(masterDataDir, body.Group, body.ActiveIds); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		catalog, _ := buildEventCatalog(masterDataDir)
		writeJSON(w, catalog)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePresets serves GET /api/master-data/presets
func handlePresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// TODO: implement preset system
	writeJSON(w, map[string]any{
		"enabled": false,
		"presets": []any{},
	})
}

// --- Banner/Event catalog builders ---

func buildBannerCatalog(momBannerPath string) (map[string]any, error) {
	data, err := readJSONFile(momBannerPath)
	if err != nil {
		return map[string]any{"enabled": false, "records": []any{}, "currentPath": momBannerPath}, nil
	}

	banners, ok := data.([]any)
	if !ok {
		return map[string]any{"enabled": false}, nil
	}

	// Load gacha medal table to determine which banners are "usable"
	medalIDs := loadGachaMedalIDs()

	// Read active IDs from content_schedule.json
	activeIDs := loadActiveBannerIDs()
	activeIDSet := map[int]bool{}
	for _, id := range activeIDs {
		activeIDSet[id] = true
	}

	// Group step-up banners by their group ID (gachaId / 1000)
	type stepEntry struct {
		row      map[string]any
		bannerID int
	}
	stepupGroups := map[int][]stepEntry{}

	var usableRecords []map[string]any
	var unusableRecords []map[string]any

	for _, raw := range banners {
		b, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if toIntFromAny(b["DestinationDomainType"]) != 1 {
			continue
		}

		momBannerID := toIntFromAny(b["MomBannerId"])
		destID := toIntFromAny(b["DestinationDomainId"])
		assetName := stringFromAny(b["BannerAssetName"])
		isStepUp := strings.HasPrefix(assetName, "step_up_")
		isChapter := strings.HasPrefix(assetName, "common_")
		hasMedal := medalIDs[destID]

		// Resolve label from lookup
		label := fmt.Sprintf("Gacha %d", destID)
		if lookupReg != nil {
			if entry := lookupReg.ResolveAnnotation("gacha_id", destID); entry != nil {
				label = entry.Label
			}
		}

		if isStepUp {
			if !hasMedal {
				unusableRecords = append(unusableRecords, map[string]any{
					"id":            momBannerID,
					"entryKey":      fmt.Sprintf("raw:%d", momBannerID),
					"gameGachaId":   destID,
					"momBannerIds":  []int{momBannerID},
					"momBannerCount": 1,
					"label":         label,
					"detail":        fmt.Sprintf("MomBanner %d → Gacha %d · %s", momBannerID, destID, assetName),
					"assetName":     assetName,
					"mode":          "step-up",
					"group":         "Premium",
					"isUsable":      false,
					"usabilityReason": "Ignored: step-up banner has no matching GachaMedal row.",
					"startDatetime": b["StartDatetime"],
					"endDatetime":   b["EndDatetime"],
					"isActive":      activeIDSet[momBannerID],
				})
				continue
			}
			groupID := destID / 1000
			stepupGroups[groupID] = append(stepupGroups[groupID], stepEntry{row: b, bannerID: momBannerID})
			continue
		}

		if !isChapter && !hasMedal {
			unusableRecords = append(unusableRecords, map[string]any{
				"id":            momBannerID,
				"entryKey":      fmt.Sprintf("raw:%d", momBannerID),
				"gameGachaId":   destID,
				"momBannerIds":  []int{momBannerID},
				"momBannerCount": 1,
				"label":         label,
				"detail":        fmt.Sprintf("MomBanner %d → Gacha %d · %s", momBannerID, destID, assetName),
				"assetName":     assetName,
				"mode":          "basic",
				"group":         "Premium",
				"isUsable":      false,
				"usabilityReason": "Ignored: premium banner has no matching GachaMedal row.",
				"startDatetime": b["StartDatetime"],
				"endDatetime":   b["EndDatetime"],
				"isActive":      activeIDSet[momBannerID],
			})
			continue
		}

		group := "Premium"
		mode := "basic"
		if isChapter {
			group = "Chapter Summons"
			mode = "chapter"
		}

		usableRecords = append(usableRecords, map[string]any{
			"id":            momBannerID,
			"entryKey":      fmt.Sprintf("direct:%d", momBannerID),
			"gameGachaId":   destID,
			"momBannerIds":  []int{momBannerID},
			"momBannerCount": 1,
			"label":         label,
			"detail":        fmt.Sprintf("MomBanner %d → Gacha %d · %s", momBannerID, destID, assetName),
			"assetName":     assetName,
			"mode":          mode,
			"group":         group,
			"isUsable":      true,
			"startDatetime": b["StartDatetime"],
			"endDatetime":   b["EndDatetime"],
			"isActive":      activeIDSet[momBannerID],
		})
	}

	// Process step-up groups
	for groupID, steps := range stepupGroups {
		sort.Slice(steps, func(i, j int) bool { return steps[i].bannerID < steps[j].bannerID })
		first := steps[0]
		ids := make([]int, len(steps))
		for i, s := range steps {
			ids[i] = s.bannerID
		}
		activeCount := 0
		for _, id := range ids {
			if activeIDSet[id] {
				activeCount++
			}
		}
		isActive := activeCount == len(ids)
		isPartial := activeCount > 0 && activeCount < len(ids)

		assetName := stringFromAny(first.row["BannerAssetName"])
		destID := toIntFromAny(first.row["DestinationDomainId"])
		label := fmt.Sprintf("Gacha %d", groupID)
		if lookupReg != nil {
			if entry := lookupReg.ResolveAnnotation("gacha_id", destID); entry != nil {
				label = entry.Label
			}
		}

		selState := "inactive"
		if isPartial {
			selState = "partial"
		} else if isActive {
			selState = "active"
		}

		detail := fmt.Sprintf("MomBanners %v → Gacha %d · %d step rows", ids, groupID, len(steps))
		if isPartial {
			detail += fmt.Sprintf(" · %d/%d steps selected", activeCount, len(steps))
		}

		usableRecords = append(usableRecords, map[string]any{
			"id":                groupID,
			"entryKey":          fmt.Sprintf("step:%d", groupID),
			"gameGachaId":       groupID,
			"momBannerIds":      ids,
			"momBannerCount":    len(ids),
			"label":             label,
			"detail":            detail,
			"assetName":         assetName,
			"mode":              "step-up",
			"group":             "Premium",
			"isUsable":          true,
			"isActive":          isActive,
			"isPartiallyActive": isPartial,
			"selectionState":    selState,
			"startDatetime":     first.row["StartDatetime"],
			"endDatetime":       first.row["EndDatetime"],
		})
	}

	// Sort usable: by group then label
	sort.Slice(usableRecords, func(i, j int) bool {
		gi := strings.ToLower(stringFromAny(usableRecords[i]["group"]))
		gj := strings.ToLower(stringFromAny(usableRecords[j]["group"]))
		if gi != gj {
			return gi < gj
		}
		return strings.ToLower(stringFromAny(usableRecords[i]["label"])) <
			strings.ToLower(stringFromAny(usableRecords[j]["label"]))
	})

	allRecords := append(usableRecords, unusableRecords...)

	return map[string]any{
		"enabled":          true,
		"currentPath":      momBannerPath,
		"records":          allRecords,
		"usableRecords":    usableRecords,
		"unusableRecords":  unusableRecords,
		"activeBannerIds":  activeIDs,
	}, nil
}

func loadGachaMedalIDs() map[int]bool {
	medalPath := filepath.Join(dataDir, "assets", "master_data", "EntityMGachaMedalTable.json")
	data, err := readJSONFile(medalPath)
	if err != nil {
		return map[int]bool{}
	}
	medals, ok := data.([]any)
	if !ok {
		return map[int]bool{}
	}
	result := map[int]bool{}
	for _, raw := range medals {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		gachaID := toIntFromAny(m["ShopTransitionGachaId"])
		if gachaID > 0 {
			result[gachaID] = true
		}
	}
	return result
}

func loadActiveBannerIDs() []int {
	sched := loadSchedule()
	schedIDs := buildScheduleIDs(sched)
	bannerIDs := schedIDs["m_mom_banner"]
	var result []int
	for id := range bannerIDs {
		result = append(result, int(id))
	}
	sort.Ints(result)
	return result
}

func saveActiveBannerIDs(momBannerPath string, activeIDs []int) error {
	// Read current schedule
	sched := loadSchedule()

	// Rebuild: find which bundles these banner IDs belong to
	activeSet := map[int64]bool{}
	for _, id := range activeIDs {
		activeSet[int64(id)] = true
	}

	// For now, just re-patch with the current schedule
	// The banner IDs will be applied via the patcher
	schedIDs := buildScheduleIDs(sched)
	schedIDs["m_mom_banner"] = activeSet

	log.Printf("[banners] saving %d active banner IDs, running patcher...", len(activeIDs))
	_, err := patcher.RunPatch(patcher.PatchOptions{
		InputPath:   pristinePath,
		OutputPath:  serverBinPath,
		ScheduleIDs: schedIDs,
	})
	if err != nil {
		return fmt.Errorf("patcher: %w", err)
	}

	go notifyLunarTear()
	return nil
}

func buildEventCatalog(masterDataDir string) (map[string]any, error) {
	eventPath := filepath.Join(masterDataDir, "EntityMEventQuestChapterTable.json")
	data, err := readJSONFile(eventPath)
	if err != nil {
		return map[string]any{"groups": map[string]any{}}, nil
	}

	events, ok := data.([]any)
	if !ok {
		return map[string]any{"groups": map[string]any{}}, nil
	}

	// Read active IDs from schedule
	activeEventIDs := loadActiveEventIDs()

	var records []map[string]any
	for _, raw := range events {
		e, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		chapterID := toIntFromAny(e["EventQuestChapterId"])

		// Resolve label from lookup
		label := fmt.Sprintf("Event Quest %d", chapterID)
		if lookupReg != nil {
			entry := lookupReg.ResolveAnnotation("event_quest_chapter_id", chapterID)
			if entry != nil {
				label = entry.Label
			}
		}

		records = append(records, map[string]any{
			"id":       chapterID,
			"label":    label,
			"detail":   fmt.Sprintf("Chapter %d", chapterID),
			"category": eventCategoryLabel(toIntFromAny(e["EventQuestType"])),
			"group":    "",
			"isSelectable": true,
		})
	}

	// Side stories
	ssPath := filepath.Join(masterDataDir, "EntityMSideStoryQuestLimitContentTable.json")
	ssData, _ := readJSONFile(ssPath)
	var ssRecords []map[string]any
	if ssList, ok := ssData.([]any); ok {
		for _, raw := range ssList {
			ss, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			ssID := toIntFromAny(ss["SideStoryQuestLimitContentId"])
			label := fmt.Sprintf("Side Story %d", ssID)
			if lookupReg != nil {
				entry := lookupReg.ResolveAnnotation("side_story_quest_id", ssID)
				if entry != nil {
					label = entry.Label
				}
			}
			ssRecords = append(ssRecords, map[string]any{
				"id":           ssID,
				"label":        label,
				"detail":       fmt.Sprintf("Side Story %d", ssID),
				"isSelectable": true,
			})
		}
	}

	activeSSIDs := []int{} // Side stories are always active in our system

	return map[string]any{
		"groups": map[string]any{
			"event_quests": map[string]any{
				"label":       "Event Quests",
				"currentPath": eventPath,
				"sourcePath":  "EntityMEventQuestChapterTable.json",
				"records":     records,
				"activeIds":   activeEventIDs,
				"categories":  buildEventCategories(records),
			},
			"side_story_quests": map[string]any{
				"label":       "Side Stories",
				"currentPath": ssPath,
				"sourcePath":  "EntityMSideStoryQuestLimitContentTable.json",
				"records":     ssRecords,
				"activeIds":   activeSSIDs,
			},
		},
	}, nil
}

func loadActiveEventIDs() []int {
	sched := loadSchedule()
	schedIDs := buildScheduleIDs(sched)
	eventIDs := schedIDs["m_event_quest_chapter"]
	var result []int
	for id := range eventIDs {
		result = append(result, int(id))
	}
	sort.Ints(result)
	return result
}

func saveEventSelection(masterDataDir, group string, activeIDs []int) error {
	sched := loadSchedule()
	schedIDs := buildScheduleIDs(sched)

	activeSet := map[int64]bool{}
	for _, id := range activeIDs {
		activeSet[int64(id)] = true
	}

	switch group {
	case "event_quests":
		schedIDs["m_event_quest_chapter"] = activeSet
	default:
		return fmt.Errorf("unknown group: %s", group)
	}

	log.Printf("[events] saving %d active %s IDs, running patcher...", len(activeIDs), group)
	_, err := patcher.RunPatch(patcher.PatchOptions{
		InputPath:   pristinePath,
		OutputPath:  serverBinPath,
		ScheduleIDs: schedIDs,
	})
	if err != nil {
		return fmt.Errorf("patcher: %w", err)
	}

	go notifyLunarTear()
	return nil
}

func eventCategoryLabel(eventType int) string {
	switch eventType {
	case 1:
		return "Event"
	case 2:
		return "Subjugation"
	case 3:
		return "Guerrilla"
	case 4:
		return "Chambers of Dusk"
	case 5:
		return "Daily"
	case 6:
		return "Labyrinth"
	case 7:
		return "Tower"
	default:
		return fmt.Sprintf("Type %d", eventType)
	}
}

func buildEventCategories(records []map[string]any) []map[string]any {
	counts := map[string]int{}
	var order []string
	for _, rec := range records {
		cat := stringFromAny(rec["category"])
		if cat == "" {
			continue
		}
		if counts[cat] == 0 {
			order = append(order, cat)
		}
		counts[cat]++
	}
	var categories []map[string]any
	for _, cat := range order {
		categories = append(categories, map[string]any{
			"id":    strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(cat, " / ", "-"), " ", "-")),
			"label": cat,
			"count": counts[cat],
		})
	}
	return categories
}

// --- Helpers ---

func readJSONFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func toIntFromAny(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	case string:
		n := 0
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return 0
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
