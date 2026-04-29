// Package bundleindex generates bundle_index.json from dumped master data JSON tables.
//
// This is the Go port of generate_bundle_index.py. It reads the EntityM*Table.json
// files from the lunar-tear server's master_data directory and groups content into
// monthly bundles by their StartDatetime.
package bundleindex

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const momBannerDomainGacha = 1

// Bundle represents a monthly content bundle.
type Bundle struct {
	Label         string  `json:"label,omitempty"`
	EventChapters []int32 `json:"event_chapters"`
	GachaIds      []int32 `json:"gacha_ids"`
	MomBannerIds  []int32 `json:"mom_banner_ids"`
	LoginBonuses  []int32 `json:"login_bonuses"`
	SideStories   []int32 `json:"side_stories"`
	ShopIds       []int32 `json:"shop_ids"`
}

// Index is the complete bundle index structure.
type Index struct {
	Bundles    map[string]*Bundle `json:"bundles"`
	Permanent  *Bundle            `json:"permanent"`
	Unreleased *Bundle            `json:"unreleased"`
}

// Result holds generation statistics.
type Result struct {
	OutputPath     string
	MonthlyBundles int
	TotalEvents    int
	TotalGacha     int
	TotalLogin     int
	TotalShops     int
	TotalSS        int
	PermanentEvents int
	UnreleasedEvents int
	UnreleasedGacha  int
	UnreleasedLogin  int
	UnreleasedShops  int
	PermanentShops   int
	DateRange      [2]string
}

func newBundle() *Bundle {
	return &Bundle{
		EventChapters: []int32{},
		GachaIds:      []int32{},
		MomBannerIds:  []int32{},
		LoginBonuses:  []int32{},
		SideStories:   []int32{},
		ShopIds:       []int32{},
	}
}

// Generate builds the bundle index from JSON table dumps in dumpDir and writes to outputPath.
func Generate(dumpDir, outputPath string) (*Result, error) {
	log.Printf("Reading master data from %s/...", dumpDir)

	// Load tables
	events := loadTable[eventChapter](dumpDir, "EntityMEventQuestChapterTable.json")
	banners := loadTable[momBanner](dumpDir, "EntityMMomBannerTable.json")
	loginBonuses := loadTable[loginBonus](dumpDir, "EntityMLoginBonusTable.json")
	ssLimit := loadTable[sideStoryLimit](dumpDir, "EntityMSideStoryQuestLimitContentTable.json")
	shops := loadTable[shop](dumpDir, "EntityMShopTable.json")

	// Build bundles
	bundles := make(map[string]*Bundle)
	permanent := newBundle()
	permanent.Label = "Permanent Content"
	unreleased := newBundle()
	unreleased.Label = "Unreleased Content"

	getBundle := func(month string) *Bundle {
		if b, ok := bundles[month]; ok {
			return b
		}
		b := newBundle()
		bundles[month] = b
		return b
	}

	// Event Quest Chapters
	chapterMonth := make(map[int32]string)
	for _, e := range events {
		month := msToMonth(e.StartDatetime)
		chapterMonth[e.EventQuestChapterId] = month

		if isUnreleased(e.StartDatetime) {
			unreleased.EventChapters = append(unreleased.EventChapters, e.EventQuestChapterId)
		} else if isPermanent(e.EndDatetime) {
			permanent.EventChapters = append(permanent.EventChapters, e.EventQuestChapterId)
			getBundle(month).EventChapters = append(getBundle(month).EventChapters, e.EventQuestChapterId)
		} else {
			getBundle(month).EventChapters = append(getBundle(month).EventChapters, e.EventQuestChapterId)
		}
	}

	// Gacha Banners (MomBanner type 1)
	// We store BOTH GachaIds (for UI display) and MomBannerIds (for patching).
	// GachaId = DestinationDomainId, MomBannerId = row primary key (column 0).
	gachaSeen := make(map[string]map[int32]bool)
	for _, b := range banners {
		if b.DestinationDomainType != momBannerDomainGacha {
			continue
		}
		month := msToMonth(b.StartDatetime)

		if isUnreleased(b.StartDatetime) {
			unreleased.GachaIds = append(unreleased.GachaIds, b.DestinationDomainId)
			unreleased.MomBannerIds = append(unreleased.MomBannerIds, b.MomBannerId)
		} else {
			// Always store MomBannerId (needed for patching)
			getBundle(month).MomBannerIds = append(getBundle(month).MomBannerIds, b.MomBannerId)
			// Deduplicate GachaIds (one gacha can have multiple banners)
			if gachaSeen[month] == nil {
				gachaSeen[month] = make(map[int32]bool)
			}
			if !gachaSeen[month][b.DestinationDomainId] {
				getBundle(month).GachaIds = append(getBundle(month).GachaIds, b.DestinationDomainId)
				gachaSeen[month][b.DestinationDomainId] = true
			}
		}
	}

	// Login Bonuses
	for _, lb := range loginBonuses {
		month := msToMonth(lb.StartDatetime)
		if isUnreleased(lb.StartDatetime) {
			unreleased.LoginBonuses = append(unreleased.LoginBonuses, lb.LoginBonusId)
		} else {
			getBundle(month).LoginBonuses = append(getBundle(month).LoginBonuses, lb.LoginBonusId)
		}
	}

	// Side Stories (linked via event chapters)
	for _, ss := range ssLimit {
		month := chapterMonth[ss.EventQuestChapterId]
		if month == "" {
			month = "unknown"
		}
		getBundle(month).SideStories = append(getBundle(month).SideStories, ss.SideStoryQuestLimitContentId)
	}

	// Shops
	for _, s := range shops {
		month := msToMonth(s.StartDatetime)
		if isUnreleased(s.StartDatetime) {
			unreleased.ShopIds = append(unreleased.ShopIds, s.ShopId)
		} else if isPermanent(s.EndDatetime) && !isUnreleased(s.StartDatetime) {
			permanent.ShopIds = append(permanent.ShopIds, s.ShopId)
		} else {
			getBundle(month).ShopIds = append(getBundle(month).ShopIds, s.ShopId)
		}
	}

	// Build output — filter out junk months
	outputBundles := make(map[string]*Bundle)
	var months []string
	for month := range bundles {
		if month == "unknown" || month == "overflow" || month == "1970-01" {
			continue
		}
		months = append(months, month)
	}
	sort.Strings(months)
	for _, month := range months {
		b := bundles[month]
		b.Label = month
		outputBundles[month] = b
	}

	idx := &Index{
		Bundles:    outputBundles,
		Permanent:  permanent,
		Unreleased: unreleased,
	}

	// Write output
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Build stats
	result := &Result{
		OutputPath:       outputPath,
		MonthlyBundles:   len(outputBundles),
		PermanentEvents:  len(permanent.EventChapters),
		UnreleasedEvents: len(unreleased.EventChapters),
		UnreleasedGacha:  len(unreleased.GachaIds),
		UnreleasedLogin:  len(unreleased.LoginBonuses),
		UnreleasedShops:  len(unreleased.ShopIds),
		PermanentShops:   len(permanent.ShopIds),
	}
	for _, b := range outputBundles {
		result.TotalEvents += len(b.EventChapters)
		result.TotalGacha += len(b.GachaIds)
		result.TotalLogin += len(b.LoginBonuses)
		result.TotalSS += len(b.SideStories)
		result.TotalShops += len(b.ShopIds)
	}
	if len(months) > 0 {
		result.DateRange = [2]string{months[0], months[len(months)-1]}
	}

	log.Printf("Bundle index generated: %s", outputPath)
	log.Printf("  Monthly bundles: %d", result.MonthlyBundles)
	log.Printf("  Events: %d (%d permanent, %d unreleased)", result.TotalEvents, result.PermanentEvents, result.UnreleasedEvents)
	log.Printf("  Gacha banners: %d (%d unreleased)", result.TotalGacha, result.UnreleasedGacha)
	log.Printf("  Login bonuses: %d (%d unreleased)", result.TotalLogin, result.UnreleasedLogin)
	log.Printf("  Shops: %d (%d permanent, %d unreleased)", result.TotalShops, result.PermanentShops, result.UnreleasedShops)
	log.Printf("  Side stories: %d", result.TotalSS)
	if len(months) > 0 {
		log.Printf("  Date range: %s -> %s", months[0], months[len(months)-1])
	}

	return result, nil
}

// --- JSON table row types ---

type eventChapter struct {
	EventQuestChapterId int32 `json:"EventQuestChapterId"`
	StartDatetime       int64 `json:"StartDatetime"`
	EndDatetime         int64 `json:"EndDatetime"`
}

type momBanner struct {
	MomBannerId           int32 `json:"MomBannerId"`
	DestinationDomainType int32 `json:"DestinationDomainType"`
	DestinationDomainId   int32 `json:"DestinationDomainId"`
	StartDatetime         int64 `json:"StartDatetime"`
}

type loginBonus struct {
	LoginBonusId  int32 `json:"LoginBonusId"`
	StartDatetime int64 `json:"StartDatetime"`
}

type sideStoryLimit struct {
	SideStoryQuestLimitContentId int32 `json:"SideStoryQuestLimitContentId"`
	EventQuestChapterId          int32 `json:"EventQuestChapterId"`
}

type shop struct {
	ShopId        int32 `json:"ShopId"`
	StartDatetime int64 `json:"StartDatetime"`
	EndDatetime   int64 `json:"EndDatetime"`
}

// --- Helpers ---

func loadTable[T any](dumpDir, filename string) []T {
	path := filepath.Join(dumpDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("  Warning: %s not found, skipping", filename)
		return nil
	}
	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		log.Printf("  Warning: %s parse error: %v", filename, err)
		return nil
	}
	return rows
}

func msToMonth(ms int64) string {
	if ms <= 0 {
		return "unknown"
	}
	t := time.UnixMilli(ms).UTC()
	if t.Year() < 2000 || t.Year() > 2100 {
		return "overflow"
	}
	return t.Format("2006-01")
}

func isUnreleased(ms int64) bool {
	if ms <= 0 {
		return false
	}
	t := time.UnixMilli(ms).UTC()
	return t.Year() >= 2099
}

func isPermanent(endMs int64) bool {
	if endMs <= 0 {
		return false
	}
	t := time.UnixMilli(endMs).UTC()
	return t.Year() >= 2090
}
