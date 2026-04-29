// Package patcher implements the MasterMemory binary patcher.
//
// It decrypts a .bin.e file, walks msgpack table blobs to patch EndDatetime
// fields, and re-encrypts the result. This replaces the Python patch_masterdata.py.
package patcher

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"mama-toolkit/internal/crypto"
	"mama-toolkit/internal/msgpackutil"

	"github.com/pierrec/lz4/v4"
	"github.com/vmihailenco/msgpack/v5"
)

// Timestamp constants matching the Python patcher
var (
	TargetEndMS  = time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC).UnixMilli()
	ExpiredEndMS = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	MinPatchMS   = time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	MaxPatchMS   = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	schedulePatchCutoffMS = time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
)

// Column describes a patchable column in a table.
type Column struct {
	Index int
	Name  string
}

// RowFilter skips rows whose column value exceeds a threshold.
type RowFilter struct {
	ColIndex int
	MaxValue int64
}

// tableConfig bundles all per-table patching metadata.
type tableConfig struct {
	Columns   []Column
	Filter    *RowFilter
	ActiveIDs map[int64]bool // nil = patch all, non-nil = filter by ID
}

// PatchColumns defines which tables and columns contain end-datetime fields.
var PatchColumns = map[string][]Column{
	"m_appeal_dialog":                  {{5, "EndDatetime"}},
	"m_big_hunt_schedule":              {{3, "ChallengeEndDatetime"}},
	"m_cage_ornament":                  {{2, "EndDatetime"}},
	"m_consumable_item_term":           {{2, "EndDatetime"}},
	"m_costume_collection_bonus":       {{6, "EndDatetime"}},
	"m_dokan":                          {{4, "EndDatetime"}},
	"m_enhance_campaign":               {{5, "EndDatetime"}},
	"m_event_quest_chapter":            {{9, "EndDatetime"}},
	"m_event_quest_daily_group":        {{2, "EndDatetime"}},
	"m_event_quest_guerrilla_free_open": {{4, "EndDatetime"}},
	"m_event_quest_labyrinth_season":   {{3, "EndDatetime"}},
	"m_event_quest_limit_content":      {{6, "EndDatetime"}},
	"m_event_quest_limit_content_deck_restriction": {{4, "EndDatetime"}},
	"m_gacha_medal":       {{4, "AutoConvertDatetime"}},
	"m_gimmick_sequence_schedule": {{2, "EndDatetime"}},
	"m_important_item_effect":     {{6, "EndDatetime"}},
	"m_login_bonus":               {{5, "EndDatetime"}, {6, "StampReceiveEndDatetime"}},
	"m_mission_pass":     {{2, "EndDatetime"}},
	"m_mission_term":     {{2, "EndDatetime"}},
	"m_mom_banner":       {{7, "EndDatetime"}},
	"m_mom_point_banner": {{4, "EndDatetime"}},
	"m_navi_cut_in":      {{4, "EndDatetime"}},
	"m_omikuji":          {{2, "EndDatetime"}},
	"m_portal_cage_access_point_function_group_schedule": {{5, "EndDatetime"}},
	"m_possession_acquisition_route": {{7, "EndDatetime"}},
	"m_premium_item":                 {{3, "EndDatetime"}},
	"m_pvp_season":                   {{3, "SeasonEndDatetime"}},
	"m_quest_bonus_term_group":       {{3, "EndDatetime"}},
	"m_quest_campaign":               {{4, "EndDatetime"}},
	"m_quest_schedule":               {{3, "EndDatetime"}},
	"m_shop":               {{10, "EndDatetime"}},
	"m_shop_item_cell_term": {{2, "EndDatetime"}},
	"m_tip":                 {{6, "EndDatetime"}},
	"m_title_flow_movie":    {{3, "EndDatetime"}},
	"m_webview_mission":     {{5, "EndDatetime"}},
	"m_webview_panel_mission": {{4, "EndDatetime"}},
}

var (
	skipTables  = map[string]bool{"m_omikuji": true}
	emptyTables = map[string]bool{"m_maintenance": true}

	tablePatchFilters = map[string]RowFilter{
		"m_gimmick_sequence_schedule": {1, schedulePatchCutoffMS},
	}
)

// ScheduleIDs maps table names to allowed row IDs for schedule-aware filtering.
type ScheduleIDs map[string]map[int64]bool

// PatchResult holds statistics for a single table patch.
type PatchResult struct {
	Table    string
	Columns  []string
	Patched  int
	Skipped  int
}

// PatchOptions configures a patch run.
type PatchOptions struct {
	InputPath    string
	OutputPath   string // defaults to InputPath if empty
	ScheduleIDs  ScheduleIDs
	DryRun       bool
}

// RunPatch executes the full patch pipeline: decrypt → patch → rebuild → re-encrypt → write.
func RunPatch(opts PatchOptions) ([]PatchResult, error) {
	if opts.OutputPath == "" {
		opts.OutputPath = opts.InputPath
	}

	// Read
	encrypted, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	log.Printf("Reading %s (%d bytes)", opts.InputPath, len(encrypted))

	// Decrypt
	decrypted, err := crypto.Decrypt(encrypted, crypto.DefaultKey, crypto.DefaultIV)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	log.Printf("Decrypted: %d bytes", len(decrypted))

	// Parse header: map[string][offset, length]
	var toc map[string][2]int
	off, err := parseHeader(decrypted, &toc)
	if err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	dataBlob := decrypted[off:]
	log.Printf("Header: %d tables, data blob: %d bytes", len(toc), len(dataBlob))

	// Patch tables
	var results []PatchResult
	newBlobs := make(map[string][]byte)

	for tname, columns := range PatchColumns {
		if skipTables[tname] {
			continue
		}
		entry, ok := toc[tname]
		if !ok {
			continue
		}

		blob := dataBlob[entry[0] : entry[0]+entry[1]]

		// Unpack ExtType
		var ext msgpack.RawMessage
		if err := msgpack.Unmarshal(blob, &ext); err != nil {
			log.Printf("WARNING: %s unmarshal failed: %v, skipping", tname, err)
			continue
		}

		// Check for ExtType(99) = LZ4 compressed
		extCode, extData, isExt := parseExtType(blob)
		if !isExt || extCode != 99 {
			log.Printf("WARNING: %s is not LZ4-compressed (ExtType 99), skipping", tname)
			continue
		}

		uncompLen, lz4Data, err := msgpackutil.ReadLZ4ExtHeader(extData)
		if err != nil {
			log.Printf("WARNING: %s LZ4 header error: %v, skipping", tname, err)
			continue
		}

		decompressed := make([]byte, uncompLen)
		n, err := lz4.UncompressBlock(lz4Data, decompressed)
		if err != nil {
			log.Printf("WARNING: %s LZ4 decompress failed: %v, skipping", tname, err)
			continue
		}
		decompressed = decompressed[:n]

		colIndices := make(map[int]bool)
		var colNames []string
		for _, c := range columns {
			colIndices[c.Index] = true
			colNames = append(colNames, c.Name)
		}

		var rowFilter *RowFilter
		if rf, ok := tablePatchFilters[tname]; ok {
			rowFilter = &rf
		}

		var activeIDs map[int64]bool
		if opts.ScheduleIDs != nil {
			if ids, ok := opts.ScheduleIDs[tname]; ok {
				activeIDs = ids
			}
		}

		patched, skipped, err := patchTableBlob(decompressed, colIndices, rowFilter, activeIDs)
		if err != nil {
			log.Printf("WARNING: %s patch error: %v, skipping", tname, err)
			continue
		}

		if patched > 0 {
			newBlobs[tname] = buildLZ4ExtBlob(decompressed)
			results = append(results, PatchResult{
				Table:   tname,
				Columns: colNames,
				Patched: patched,
				Skipped: skipped,
			})
		}
	}

	// Empty tables
	for tname := range emptyTables {
		if _, ok := toc[tname]; ok {
			emptyArray, _ := msgpack.Marshal([]interface{}{})
			newBlobs[tname] = emptyArray
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Table < results[j].Table })

	if opts.DryRun {
		log.Println("[DRY RUN] Skipping rebuild and write")
		return results, nil
	}

	// Rebuild binary
	rebuilt, err := rebuildBinary(toc, dataBlob, newBlobs)
	if err != nil {
		return nil, fmt.Errorf("rebuild: %w", err)
	}

	// Re-encrypt
	reEncrypted, err := crypto.Encrypt(rebuilt, crypto.DefaultKey, crypto.DefaultIV)
	if err != nil {
		return nil, fmt.Errorf("re-encrypt: %w", err)
	}

	// Write
	if err := os.WriteFile(opts.OutputPath, reEncrypted, 0644); err != nil {
		return nil, fmt.Errorf("write output: %w", err)
	}
	log.Printf("Patched binary written to %s (%d bytes)", opts.OutputPath, len(reEncrypted))

	return results, nil
}

// parseHeader parses the msgpack header (dict of table→[offset, size]).
// Uses a streaming decoder via bytes.Reader to find the exact byte boundary
// between header and data blob, avoiding re-marshal discrepancies.
func parseHeader(data []byte, toc *map[string][2]int) (int, error) {
	r := bytes.NewReader(data)
	dec := msgpack.NewDecoder(r)
	var raw map[string][2]int
	if err := dec.Decode(&raw); err != nil {
		return 0, fmt.Errorf("decode header: %w", err)
	}
	*toc = raw
	// bytes.Reader.Len() tells us how many bytes remain unread
	headerLen := len(data) - r.Len()
	return headerLen, nil
}

// rebuildBinary reassembles the data blob with new blobs for patched tables.
// The header is written using raw msgpack encoding to match the C# format:
// map16/map32 with str8/str16 keys and fixarray [int32, int32] values.
func rebuildBinary(toc map[string][2]int, originalBlob []byte, newBlobs map[string][]byte) ([]byte, error) {
	type tableEntry struct {
		name   string
		offset int
		size   int
	}
	sorted := make([]tableEntry, 0, len(toc))
	for name, entry := range toc {
		sorted = append(sorted, tableEntry{name, entry[0], entry[1]})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].offset < sorted[j].offset })

	newToc := make(map[string][2]int, len(toc))
	var blobParts []byte
	currentOffset := 0

	for _, te := range sorted {
		var part []byte
		if nb, ok := newBlobs[te.name]; ok {
			part = nb
		} else {
			part = originalBlob[te.offset : te.offset+te.size]
		}
		newToc[te.name] = [2]int{currentOffset, len(part)}
		blobParts = append(blobParts, part...)
		currentOffset += len(part)
	}

	// Encode header manually using raw msgpack to match C# MasterMemory format.
	// C# uses str8 for keys and fixarray with int32 values.
	headerBytes := encodeHeaderRaw(newToc)

	result := make([]byte, len(headerBytes)+len(blobParts))
	copy(result, headerBytes)
	copy(result[len(headerBytes):], blobParts)

	log.Printf("Rebuilt: header %d bytes, blob %d bytes, total %d bytes",
		len(headerBytes), len(blobParts), len(result))

	return result, nil
}

// encodeHeaderRaw encodes the TOC map using the same msgpack format that
// C# MessagePack / MasterMemory uses:
//   - map16 (0xde) for the outer map
//   - str8 (0xd9) for string keys
//   - fixarray(2) + int32 (0xd2) for the [offset, size] pairs
func encodeHeaderRaw(toc map[string][2]int) []byte {
	var buf bytes.Buffer

	// Sort keys to get deterministic output
	keys := make([]string, 0, len(toc))
	for k := range toc {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	n := len(keys)
	if n <= 0x0f {
		buf.WriteByte(0x80 | byte(n)) // fixmap
	} else if n <= 0xffff {
		buf.WriteByte(0xde) // map16
		b := [2]byte{}
		binary.BigEndian.PutUint16(b[:], uint16(n))
		buf.Write(b[:])
	} else {
		buf.WriteByte(0xdf) // map32
		b := [4]byte{}
		binary.BigEndian.PutUint32(b[:], uint32(n))
		buf.Write(b[:])
	}

	for _, key := range keys {
		entry := toc[key]
		// String key: use str8 (0xd9) for keys up to 255 bytes
		kBytes := []byte(key)
		kLen := len(kBytes)
		if kLen <= 31 {
			buf.WriteByte(0xa0 | byte(kLen)) // fixstr
		} else if kLen <= 0xff {
			buf.WriteByte(0xd9) // str8
			buf.WriteByte(byte(kLen))
		} else {
			buf.WriteByte(0xda) // str16
			b := [2]byte{}
			binary.BigEndian.PutUint16(b[:], uint16(kLen))
			buf.Write(b[:])
		}
		buf.Write(kBytes)

		// Value: fixarray(2) with two int32 values
		buf.WriteByte(0x92) // fixarray, length 2

		// offset as int32
		buf.WriteByte(0xd2)
		b := [4]byte{}
		binary.BigEndian.PutUint32(b[:], uint32(entry[0]))
		buf.Write(b[:])

		// size as int32
		buf.WriteByte(0xd2)
		binary.BigEndian.PutUint32(b[:], uint32(entry[1]))
		buf.Write(b[:])
	}

	return buf.Bytes()
}



// parseExtType checks if raw msgpack bytes represent an ExtType and returns its code and data.
func parseExtType(blob []byte) (int8, []byte, bool) {
	tag := blob[0]
	switch {
	case tag == 0xd4: // fixext1
		return int8(blob[1]), blob[2:3], true
	case tag == 0xd5: // fixext2
		return int8(blob[1]), blob[2:4], true
	case tag == 0xd6: // fixext4
		return int8(blob[1]), blob[2:6], true
	case tag == 0xd7: // fixext8
		return int8(blob[1]), blob[2:10], true
	case tag == 0xd8: // fixext16
		return int8(blob[1]), blob[2:18], true
	case tag == 0xc7: // ext8
		n := int(blob[1])
		return int8(blob[2]), blob[3 : 3+n], true
	case tag == 0xc8: // ext16
		n := int(binary.BigEndian.Uint16(blob[1:3]))
		return int8(blob[3]), blob[4 : 4+n], true
	case tag == 0xc9: // ext32
		n := int(binary.BigEndian.Uint32(blob[1:5]))
		return int8(blob[5]), blob[6 : 6+n], true
	}
	return 0, nil, false
}

// buildLZ4ExtBlob compresses data and wraps it as a msgpack ExtType(99).
func buildLZ4ExtBlob(data []byte) []byte {
	// LZ4 compress
	maxDst := lz4.CompressBlockBound(len(data))
	compressed := make([]byte, maxDst)
	n, err := lz4.CompressBlock(data, compressed, nil)
	if err != nil || n == 0 {
		// Fallback: store uncompressed (shouldn't happen)
		compressed = data
		n = len(data)
	}
	compressed = compressed[:n]

	// Build ExtType(99) payload: [int32 uncompressed_len][lz4 data]
	sizeHeader := make([]byte, 5)
	sizeHeader[0] = 0xd2 // int32
	binary.BigEndian.PutUint32(sizeHeader[1:], uint32(len(data)))

	payload := append(sizeHeader, compressed...)
	result, _ := msgpack.Marshal(msgpack.RawMessage(wrapExtType(99, payload)))
	return result
}

// wrapExtType encodes an ExtType with the given code and data.
func wrapExtType(code int8, data []byte) []byte {
	n := len(data)
	switch {
	case n == 1:
		return []byte{0xd4, byte(code), data[0]}
	case n == 2:
		return append([]byte{0xd5, byte(code)}, data...)
	case n == 4:
		return append([]byte{0xd6, byte(code)}, data...)
	case n == 8:
		return append([]byte{0xd7, byte(code)}, data...)
	case n == 16:
		return append([]byte{0xd8, byte(code)}, data...)
	case n <= 0xff:
		return append([]byte{0xc7, byte(n), byte(code)}, data...)
	case n <= 0xffff:
		buf := []byte{0xc8, 0, 0, byte(code)}
		binary.BigEndian.PutUint16(buf[1:3], uint16(n))
		return append(buf, data...)
	default:
		buf := []byte{0xc9, 0, 0, 0, 0, byte(code)}
		binary.BigEndian.PutUint32(buf[1:5], uint32(n))
		return append(buf, data...)
	}
}

// patchTableBlob patches int64 datetime values in-place within decompressed table data.
func patchTableBlob(blob []byte, colIndices map[int]bool, rowFilter *RowFilter, activeIDs map[int64]bool) (int, int, error) {
	rowCount, pos, err := msgpackutil.ReadArrayLen(blob, 0)
	if err != nil {
		return 0, 0, err
	}

	patched := 0
	skipped := 0

	for i := 0; i < rowCount; i++ {
		colCount, p, err := msgpackutil.ReadArrayLen(blob, pos)
		if err != nil {
			return patched, skipped, fmt.Errorf("row %d: %w", i, err)
		}

		skipRow := false
		var rowID int64

		if activeIDs != nil && colCount > 0 {
			rowID, _ = msgpackutil.ReadInt(blob, p)
		}

		if rowFilter != nil {
			fp := p
			for ci := 0; ci < rowFilter.ColIndex+1 && ci < colCount; ci++ {
				if ci == rowFilter.ColIndex && blob[fp] == 0xd3 {
					val := int64(binary.BigEndian.Uint64(blob[fp+1:]))
					if val >= rowFilter.MaxValue {
						skipRow = true
					}
					break
				}
				fp, err = msgpackutil.SkipValue(blob, fp)
				if err != nil {
					return patched, skipped, fmt.Errorf("row %d filter skip: %w", i, err)
				}
			}
		}

		if skipRow {
			skipped++
			for ci := 0; ci < colCount; ci++ {
				p, err = msgpackutil.SkipValue(blob, p)
				if err != nil {
					return patched, skipped, fmt.Errorf("row %d skip: %w", i, err)
				}
			}
		} else {
			isActive := true
			if activeIDs != nil && !activeIDs[rowID] {
				isActive = false
			}

			for ci := 0; ci < colCount; ci++ {
				if colIndices[ci] && blob[p] == 0xd3 {
					val := int64(binary.BigEndian.Uint64(blob[p+1:]))
					if (val >= MinPatchMS && val <= MaxPatchMS) || val == TargetEndMS || val == ExpiredEndMS {
						patchVal := TargetEndMS
						if !isActive {
							patchVal = ExpiredEndMS
						}
						binary.BigEndian.PutUint64(blob[p+1:], uint64(patchVal))
						patched++
					}
				}
				p, err = msgpackutil.SkipValue(blob, p)
				if err != nil {
					return patched, skipped, fmt.Errorf("row %d col %d: %w", i, ci, err)
				}
			}
		}
		pos = p
	}

	return patched, skipped, nil
}
