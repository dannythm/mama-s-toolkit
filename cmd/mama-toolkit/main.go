package main

import (
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mama "mama-toolkit"
	"mama-toolkit/internal/bundleindex"
	"mama-toolkit/internal/editor"
	"mama-toolkit/internal/lookup"
	"mama-toolkit/internal/patcher"
)

// --- Data types ---

type Bundle struct {
	EventChapters []int32 `json:"event_chapters"`
	GachaIds      []int32 `json:"gacha_ids"`
	MomBannerIds  []int32 `json:"mom_banner_ids"`
	LoginBonuses  []int32 `json:"login_bonuses"`
	SideStories   []int32 `json:"side_stories"`
	ShopIds       []int32 `json:"shop_ids"`
}

type BundleIndex struct {
	Bundles    map[string]Bundle `json:"bundles"`
	Permanent  Bundle            `json:"permanent"`
	Unreleased Bundle            `json:"unreleased"`
}

type ContentSchedule struct {
	ActiveBundles     []string `json:"active_bundles"`
	UnreleasedEnabled bool     `json:"unreleased_enabled"`
}

// --- Globals ---

var (
	dataDir          string
	bundleIdxPath    string
	schedulePath     string
	serverBinPath    string
	pristinePath     string
	adminWebhookURL  string
	adminBearerToken string
	bundleIndex      *BundleIndex

	// Editor subsystem
	dbEditor      *editor.Editor
	lookupReg     *lookup.Registry
	cascadeCtx    *editor.CascadeContext
	deckViewer    *editor.DeckViewer
)

func main() {
	dataDirFlag := flag.String("data-dir", "../lunar-tear/server", "Path to lunar-tear server directory")
	port := flag.String("port", "8081", "Port to serve the web UI on")
	webhook := flag.String("webhook", "http://localhost:8082/api/admin/master-data/reload", "Webhook URL to trigger master-data reload in lunar-tear")
	adminToken := flag.String("admin-token", "", "Bearer token for the admin webhook (default: LUNAR_ADMIN_TOKEN env var)")
	flag.Parse()

	dataDir = *dataDirFlag
	bundleIdxPath = filepath.Join(dataDir, "assets", "bundle_index.json")
	schedulePath = filepath.Join(dataDir, "assets", "release", "content_schedule.json")
	serverBinPath = filepath.Join(dataDir, "assets", "release", "20240404193219.bin.e")
	pristinePath = serverBinPath + ".pristine"
	adminWebhookURL = *webhook

	// Resolve admin token: flag > env var
	adminBearerToken = strings.TrimSpace(*adminToken)
	if adminBearerToken == "" {
		adminBearerToken = strings.TrimSpace(os.Getenv("LUNAR_ADMIN_TOKEN"))
	}
	if adminBearerToken == "" {
		log.Println("WARNING: no admin token set (--admin-token or LUNAR_ADMIN_TOKEN). Reload webhook calls will be skipped.")
	} else {
		log.Printf("Admin reload webhook: %s (token set)", adminWebhookURL)
	}

	// Create pristine backup on first run
	if _, err := os.Stat(pristinePath); os.IsNotExist(err) {
		log.Printf("Creating pristine backup: %s", pristinePath)
		src, err := os.ReadFile(serverBinPath)
		if err != nil {
			log.Fatalf("Failed to read master data for backup: %v", err)
		}
		if err := os.WriteFile(pristinePath, src, 0644); err != nil {
			log.Fatalf("Failed to create pristine backup: %v", err)
		}
	}

	// Auto-generate bundle index if missing
	masterDataDir := filepath.Join(dataDir, "assets", "master_data")
	if _, err := os.Stat(bundleIdxPath); os.IsNotExist(err) {
		if _, err := os.Stat(masterDataDir); err == nil {
			log.Println("bundle_index.json not found — generating from master data...")
			if _, err := bundleindex.Generate(masterDataDir, bundleIdxPath); err != nil {
				log.Printf("WARNING: Failed to generate bundle index: %v", err)
			}
		} else {
			log.Printf("WARNING: Neither bundle_index.json nor master_data dir found")
		}
	}

	// Load bundle index
	if err := loadBundleIndex(); err != nil {
		log.Printf("WARNING: Failed to load bundle index from %s: %v", bundleIdxPath, err)
		log.Println("Bundle-based content management will be unavailable.")
		bundleIndex = &BundleIndex{Bundles: map[string]Bundle{}}
	}

	// Initialize editor subsystem
	dbPath := filepath.Join(dataDir, "db", "game.db")
	if _, err := os.Stat(dbPath); err == nil {
		var edErr error
		dbEditor, edErr = editor.NewEditor(dbPath)
		if edErr != nil {
			log.Printf("WARNING: editor init failed: %v", edErr)
		} else {
			log.Printf("Editor initialized: %s", dbPath)
		}
	} else {
		log.Printf("No game.db found at %s — editor will be unavailable", dbPath)
	}

	// Initialize lookup resolver from Engels output
	// Look for output/ in the toolkit directory, or a sibling Engels/output or example-output
	exeDir := filepath.Dir(os.Args[0])
	engelsOutputDir := ""
	for _, candidate := range []string{
		filepath.Join(exeDir, "output"),
		"output",
		filepath.Join(exeDir, "..", "Engels", "output"),
		filepath.Join(exeDir, "..", "Engels", "example-output"),
		filepath.Join("..", "Engels", "output"),
		filepath.Join("..", "Engels", "example-output"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			engelsOutputDir = candidate
			break
		}
	}
	if engelsOutputDir != "" {
		lookupReg = lookup.NewRegistry(engelsOutputDir)
		cascadeCtx = editor.NewCascadeContext(engelsOutputDir)
	} else {
		log.Println("No Engels output dir found — lookups and cascade disabled")
		lookupReg = lookup.NewRegistry("")
		cascadeCtx = editor.NewCascadeContext("")
	}

	// Serve embedded web UI and filesystem images
	webContent, _ := fs.Sub(mama.WebFS, "web")

	// Images are served from the filesystem (too large to embed: ~500MB)
	imagesDir := filepath.Join(filepath.Dir(os.Args[0]), "images")
	for _, candidate := range []string{
		imagesDir,
		"images",
		filepath.Join("..", "Mama-s-toolbox", "images"),
		filepath.Join(exeDir, "..", "Mama-s-toolbox", "images"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			imagesDir = candidate
			break
		}
	}

	// Initialize deck viewer
	if dbEditor != nil {
		deckViewer = editor.NewDeckViewer(dbEditor, cascadeCtx, imagesDir)
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(imagesDir))))
	mux.Handle("/styles.css", http.FileServer(http.FS(webContent)))
	mux.Handle("/app.js", http.FileServer(http.FS(webContent)))

	// Content manager API endpoints
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/schedule", handleSchedule)
	mux.HandleFunc("/api/bundles", handleBundles)
	mux.HandleFunc("/api/bundles/regenerate", handleBundlesRegenerate)

	// Mama's Toolbox API endpoints (save editor)
	mux.HandleFunc("/api/overview", handleOverview)
	mux.HandleFunc("/api/user/", handleUserRoutes)
	mux.HandleFunc("/api/table/", handleTableRoutes)
	mux.HandleFunc("/api/lookups/", handleLookups)
	mux.HandleFunc("/api/master-data/gacha-banners", handleGachaBanners)
	mux.HandleFunc("/api/master-data/events", handleEvents)
	mux.HandleFunc("/api/master-data/presets", handlePresets)

	// Index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := mama.WebFS.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	log.Printf("Mama's Toolkit listening on :%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, mux))
}

func loadBundleIndex() error {
	data, err := os.ReadFile(bundleIdxPath)
	if err != nil {
		return err
	}
	var idx BundleIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return err
	}
	bundleIndex = &idx
	return nil
}

func loadSchedule() ContentSchedule {
	var sched ContentSchedule
	data, err := os.ReadFile(schedulePath)
	if err == nil {
		json.Unmarshal(data, &sched)
	}
	return sched
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sched := loadSchedule()
	stats := calcStats(sched)
	writeJSON(w, map[string]any{
		"stats":    stats,
		"schedule": sched,
	})
}

func handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, loadSchedule())
	case http.MethodPost:
		var sched ContentSchedule
		if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		data, _ := json.MarshalIndent(sched, "", "  ")
		os.WriteFile(schedulePath, data, 0644)

		// Build schedule IDs from bundle index
		schedIDs := buildScheduleIDs(sched)

		log.Println("Running patch (Go)...")
		start := time.Now()
		results, err := patcher.RunPatch(patcher.PatchOptions{
			InputPath:   pristinePath,
			OutputPath:  serverBinPath,
			ScheduleIDs: schedIDs,
		})
		if err != nil {
			log.Printf("Patcher failed: %v", err)
			http.Error(w, "patcher failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("Patcher completed in %v (%d tables patched)", time.Since(start), len(results))

		// Trigger master-data reload
		go notifyLunarTear()

		writeJSON(w, map[string]any{
			"ok":      true,
			"stats":   calcStats(sched),
			"results": results,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleBundles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	months := make([]string, 0, len(bundleIndex.Bundles))
	for m := range bundleIndex.Bundles {
		months = append(months, m)
	}
	sort.Strings(months)

	type bundleInfo struct {
		Month          string  `json:"month"`
		EventCount     int     `json:"event_count"`
		GachaCount     int     `json:"gacha_count"`
		LoginCount     int     `json:"login_count"`
		SideStoryCount int     `json:"side_story_count"`
		ShopCount      int     `json:"shop_count"`
		EventChapters  []int32 `json:"event_chapters"`
		GachaIds       []int32 `json:"gacha_ids"`
		LoginBonuses   []int32 `json:"login_bonuses"`
		SideStories    []int32 `json:"side_stories"`
		ShopIds        []int32 `json:"shop_ids"`
	}

	bundles := make([]bundleInfo, 0, len(months))
	for _, m := range months {
		b := bundleIndex.Bundles[m]
		bundles = append(bundles, bundleInfo{
			Month:          m,
			EventCount:     len(b.EventChapters),
			GachaCount:     len(b.GachaIds),
			LoginCount:     len(b.LoginBonuses),
			SideStoryCount: len(b.SideStories),
			ShopCount:      len(b.ShopIds),
			EventChapters:  b.EventChapters,
			GachaIds:       b.GachaIds,
			LoginBonuses:   b.LoginBonuses,
			SideStories:    b.SideStories,
			ShopIds:        b.ShopIds,
		})
	}

	writeJSON(w, map[string]any{
		"bundles":    bundles,
		"permanent":  bundleIndex.Permanent,
		"unreleased": bundleIndex.Unreleased,
	})
}

func handleBundlesRegenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	masterDataDir := filepath.Join(dataDir, "assets", "master_data")
	result, err := bundleindex.Generate(masterDataDir, bundleIdxPath)
	if err != nil {
		http.Error(w, "generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Reload into memory
	if err := loadBundleIndex(); err != nil {
		log.Printf("WARNING: Failed to reload bundle index: %v", err)
	}

	writeJSON(w, map[string]any{
		"ok":     true,
		"result": result,
	})
}

func buildScheduleIDs(sched ContentSchedule) patcher.ScheduleIDs {
	allowedEvents := int32Set(bundleIndex.Permanent.EventChapters)
	// Use MomBannerIds for m_mom_banner patching (column 0 = MomBannerId, NOT GachaId)
	allowedBanners := int32Set(bundleIndex.Permanent.MomBannerIds)
	allowedLogin := int32Set(bundleIndex.Permanent.LoginBonuses)
	allowedShops := int32Set(bundleIndex.Permanent.ShopIds)

	if sched.UnreleasedEnabled {
		mergeInt32Set(allowedEvents, bundleIndex.Unreleased.EventChapters)
		mergeInt32Set(allowedBanners, bundleIndex.Unreleased.MomBannerIds)
		mergeInt32Set(allowedLogin, bundleIndex.Unreleased.LoginBonuses)
		mergeInt32Set(allowedShops, bundleIndex.Unreleased.ShopIds)
	}

	for _, bid := range sched.ActiveBundles {
		if b, ok := bundleIndex.Bundles[bid]; ok {
			mergeInt32Set(allowedEvents, b.EventChapters)
			mergeInt32Set(allowedBanners, b.MomBannerIds)
			mergeInt32Set(allowedLogin, b.LoginBonuses)
			mergeInt32Set(allowedShops, b.ShopIds)
		}
	}

	// Failsafe: if no banners selected, force the Automata gacha
	// MomBannerIds 4,5 → GachaIds 45,46 (the NieR:Automata fallback banners)
	if len(allowedBanners) == 0 {
		allowedBanners[4] = true
		allowedBanners[5] = true
		log.Println("WARNING: 0 banners selected. Enabled fallback Automata banners (MomBannerId 4, 5).")
	}

	return patcher.ScheduleIDs{
		"m_event_quest_chapter": allowedEvents,
		"m_mom_banner":          allowedBanners,
		"m_login_bonus":         allowedLogin,
		"m_shop":                allowedShops,
	}
}

func calcStats(sched ContentSchedule) map[string]int {
	events, gacha, login, sideStories, shops := 0, 0, 0, 0, 0
	for _, m := range sched.ActiveBundles {
		if b, ok := bundleIndex.Bundles[m]; ok {
			events += len(b.EventChapters)
			gacha += len(b.GachaIds)
			login += len(b.LoginBonuses)
			sideStories += len(b.SideStories)
			shops += len(b.ShopIds)
		}
	}
	if sched.UnreleasedEnabled {
		events += len(bundleIndex.Unreleased.EventChapters)
		gacha += len(bundleIndex.Unreleased.GachaIds)
		login += len(bundleIndex.Unreleased.LoginBonuses)
		sideStories += len(bundleIndex.Unreleased.SideStories)
		shops += len(bundleIndex.Unreleased.ShopIds)
	}
	return map[string]int{
		"active_bundles":        len(sched.ActiveBundles),
		"active_gacha_entries":  gacha,
		"total_bundles":         len(bundleIndex.Bundles),
		"permanent_event_count": len(bundleIndex.Permanent.EventChapters),
		"active_login":          login,
		"active_side_stories":   sideStories,
		"active_shops":          shops,
	}
}

func notifyLunarTear() {
	if adminBearerToken == "" {
		log.Println("[reload] skipped: no admin token configured")
		return
	}
	req, err := http.NewRequest(http.MethodPost, adminWebhookURL, nil)
	if err != nil {
		log.Printf("[reload] failed to create request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+adminBearerToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[reload] webhook POST failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		log.Println("[reload] lunar-tear master-data reloaded successfully")
	} else {
		log.Printf("[reload] webhook returned status %d", resp.StatusCode)
	}
}

func int32Set(ids []int32) map[int64]bool {
	s := make(map[int64]bool, len(ids))
	for _, id := range ids {
		s[int64(id)] = true
	}
	return s
}

func mergeInt32Set(dst map[int64]bool, ids []int32) {
	for _, id := range ids {
		dst[int64(id)] = true
	}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}
