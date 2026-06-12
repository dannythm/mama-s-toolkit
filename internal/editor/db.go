// Package editor provides a SQLite-backed save editor for NieR Re[in].
//
// It wraps the game's SQLite database and provides CRUD operations
// with user scoping, annotation resolution, and cascade logic.
package editor

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// ColumnInfo describes a single column in a table.
type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"notNull"`
	DefaultSQL string `json:"defaultSql"`
	IsPrimary  bool   `json:"isPrimary"`
}

// TableSchema describes the shape of a table.
type TableSchema struct {
	Name       string       `json:"name"`
	Columns    []ColumnInfo `json:"columns"`
	PrimaryKey []string     `json:"primaryKey"`
}

// TableGroup groups related tables for the UI sidebar.
type TableGroup struct {
	Key    string   `json:"key"`
	Label  string   `json:"label"`
	Tables []string `json:"tables"`
}

// Editor is the main database editor instance.
type Editor struct {
	DBPath      string
	Schema      map[string]*TableSchema
	TableGroups []TableGroup
}

// NewEditor opens the database, loads the schema, and returns an Editor.
func NewEditor(dbPath string) (*Editor, error) {
	e := &Editor{DBPath: dbPath}

	schema, err := e.loadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	e.Schema = schema
	e.TableGroups = buildTableGroups(schema)
	log.Printf("[editor] loaded schema: %d tables from %s", len(schema), dbPath)
	return e, nil
}

// Connect opens a new connection to the database.
func (e *Editor) Connect() (*sql.DB, error) {
	return sql.Open("sqlite", e.DBPath)
}

// loadSchema discovers all tables, columns, and primary keys.
func (e *Editor) loadSchema() (map[string]*TableSchema, error) {
	db, err := e.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schema := make(map[string]*TableSchema)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		ts, err := e.loadTableSchema(db, name)
		if err != nil {
			log.Printf("[editor] skip table %s: %v", name, err)
			continue
		}
		schema[name] = ts
	}
	return schema, nil
}

func (e *Editor) loadTableSchema(db *sql.DB, table string) (*TableSchema, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ts := &TableSchema{Name: table}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			continue
		}
		col := ColumnInfo{
			Name:      name,
			Type:      colType,
			NotNull:   notNull != 0,
			IsPrimary: pk > 0,
		}
		if dflt.Valid {
			col.DefaultSQL = dflt.String
		}
		ts.Columns = append(ts.Columns, col)
		if pk > 0 {
			ts.PrimaryKey = append(ts.PrimaryKey, name)
		}
	}
	return ts, nil
}

// Overview returns database statistics.
func (e *Editor) Overview() (map[string]any, error) {
	db, err := e.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rowCounts := make(map[string]int)
	for table := range e.Schema {
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err == nil {
			rowCounts[table] = count
		}
	}

	users, err := e.listUsersDB(db)
	if err != nil {
		users = nil
	}

	schemaJSON := make(map[string]any)
	for name, ts := range e.Schema {
		schemaJSON[name] = ts
	}

	groupsJSON := make([]any, len(e.TableGroups))
	for i, g := range e.TableGroups {
		groupsJSON[i] = map[string]any{"key": g.Key, "label": g.Label, "tables": g.Tables}
	}

	return map[string]any{
		"dbPath":      e.DBPath,
		"tableCount":  len(e.Schema),
		"userCount":   len(users),
		"rowCounts":   rowCounts,
		"schema":      schemaJSON,
		"tableGroups": groupsJSON,
		"users":       users,
	}, nil
}

// ListUsers returns basic user profiles.
func (e *Editor) ListUsers() ([]map[string]any, error) {
	db, err := e.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return e.listUsersDB(db)
}

func (e *Editor) listUsersDB(db *sql.DB) ([]map[string]any, error) {
	query := `
		SELECT u.user_id, u.uuid, u.player_id,
		       COALESCE(p.name, '') AS name,
		       COALESCE(p.message, '') AS message,
		       COALESCE(s.level, 0) AS level,
		       COALESCE(s.exp, 0) AS exp,
		       COALESCE(g.paid_gem, 0) AS paid_gem,
		       COALESCE(g.free_gem, 0) AS free_gem,
		       COALESCE(q.completed_quests, 0) AS completed_quests
		FROM users u
		LEFT JOIN user_profile p ON p.user_id = u.user_id
		LEFT JOIN user_status s ON s.user_id = u.user_id
		LEFT JOIN user_gem g ON g.user_id = u.user_id
		LEFT JOIN (
			SELECT user_id, COUNT(*) AS completed_quests
			FROM user_quests
			WHERE clear_count > 0 OR last_clear_datetime > 0
			GROUP BY user_id
		) q ON q.user_id = u.user_id
		ORDER BY u.user_id
	`
	rows, err := db.Query(query)
	if err != nil {
		// Fallback: try without join
		return e.listUsersSimple(db)
	}
	defer rows.Close()

	var users []map[string]any
	for rows.Next() {
		var userID int64
		var uuid string
		var playerID int64
		var name, message string
		var level, exp int
		var paidGem, freeGem int
		var completedQuests int
		if err := rows.Scan(&userID, &uuid, &playerID, &name, &message, &level, &exp, &paidGem, &freeGem, &completedQuests); err != nil {
			log.Printf("[editor] scan user row: %v", err)
			continue
		}
		users = append(users, map[string]any{
			"userId":           userID,
			"uuid":             uuid,
			"playerId":         playerID,
			"name":             name,
			"message":          message,
			"level":            level,
			"exp":              exp,
			"paidGem":          paidGem,
			"freeGem":          freeGem,
			"completedQuests":  completedQuests,
		})
	}
	return users, nil
}

func (e *Editor) listUsersSimple(db *sql.DB) ([]map[string]any, error) {
	if _, ok := e.Schema["user_profile"]; !ok {
		return nil, nil
	}
	rows, err := db.Query("SELECT user_id, name, message FROM user_profile ORDER BY user_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []map[string]any
	for rows.Next() {
		var userID, name, message string
		if err := rows.Scan(&userID, &name, &message); err != nil {
			continue
		}
		users = append(users, map[string]any{
			"userId":  userID,
			"name":    name,
			"message": message,
			"level":   1,
		})
	}
	return users, nil
}

// UserSummary returns a detailed user profile.
func (e *Editor) UserSummary(userID string) (map[string]any, error) {
	db, err := e.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT u.user_id, u.uuid, u.player_id,
		       COALESCE(p.name, '') AS name,
		       COALESCE(p.message, '') AS message,
		       COALESCE(p.favorite_costume_id, 0) AS favorite_costume_id,
		       COALESCE(s.level, 0) AS level,
		       COALESCE(s.exp, 0) AS exp,
		       COALESCE(g.paid_gem, 0) AS paid_gem,
		       COALESCE(g.free_gem, 0) AS free_gem,
		       COALESCE(u.latest_version, 0) AS latest_version
		FROM users u
		LEFT JOIN user_profile p ON p.user_id = u.user_id
		LEFT JOIN user_status s ON s.user_id = u.user_id
		LEFT JOIN user_gem g ON g.user_id = u.user_id
		WHERE u.user_id = ?
	`
	var uid int64
	var uuid string
	var pid int64
	var name, message string
	var favCostume int64
	var level, exp int
	var paidGem, freeGem int
	var latestVersion int64

	err = db.QueryRow(query, userID).Scan(
		&uid, &uuid, &pid,
		&name, &message, &favCostume,
		&level, &exp, &paidGem, &freeGem, &latestVersion,
	)
	if err != nil {
		return map[string]any{"userId": userID}, nil
	}

	result := map[string]any{
		"userId":            uid,
		"uuid":              uuid,
		"playerId":          pid,
		"name":              name,
		"message":           message,
		"favoriteCostumeId": fmt.Sprintf("%d", favCostume),
		"level":             level,
		"exp":               exp,
		"paidGem":           paidGem,
		"freeGem":           freeGem,
		"latestVersion":     latestVersion,
	}

	// Counts
	for _, table := range []string{"user_quests", "user_costumes", "user_weapons", "user_companions"} {
		if _, ok := e.Schema[table]; ok {
			var count int
			err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = ?", table), userID).Scan(&count)
			if err == nil {
				result[table+"Count"] = count
			}
		}
	}

	// Completed quests
	if _, ok := e.Schema["user_quests"]; ok {
		var completed int
		db.QueryRow("SELECT COUNT(*) FROM user_quests WHERE user_id = ? AND (clear_count > 0 OR last_clear_datetime > 0)", userID).Scan(&completed)
		result["completedQuests"] = completed
	}

	return result, nil
}

// DeleteUser removes a user and all linked rows from user-scoped tables.
func (e *Editor) DeleteUser(userID string) error {
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

	for table, ts := range e.Schema {
		if !isUserScoped(ts) {
			continue
		}
		_, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", table), userID)
		if err != nil {
			log.Printf("[editor] delete from %s: %v", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("[editor] deleted user %s from all user-scoped tables", userID)
	return nil
}

// InvalidateSessions deletes active sessions for a user.
func (e *Editor) InvalidateSessions(userID string) (int, error) {
	db, err := e.Connect()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if _, ok := e.Schema["user_sessions"]; !ok {
		return 0, nil
	}

	result, err := db.Exec("DELETE FROM user_sessions WHERE user_id = ?", userID)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// TableRows reads all rows from a table, optionally filtering by user_id.
func (e *Editor) TableRows(table, userID string) ([]map[string]any, error) {
	ts, ok := e.Schema[table]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", table)
	}

	db, err := e.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := fmt.Sprintf("SELECT * FROM %s", table)
	var args []any
	if userID != "" && isUserScoped(ts) {
		query += " WHERE user_id = ?"
		args = append(args, userID)
	}
	query += " LIMIT 10000"

	sqlRows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	columns, _ := sqlRows.Columns()
	var rows []map[string]any
	for sqlRows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := sqlRows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = normalizeValue(values[i])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// UpsertRow inserts or replaces a row in a table.
func (e *Editor) UpsertRow(table string, row map[string]any) error {
	ts, ok := e.Schema[table]
	if !ok {
		return fmt.Errorf("table not found: %s", table)
	}

	db, err := e.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	return e.upsertRowDB(db, table, ts, row)
}

func (e *Editor) upsertRowDB(db *sql.DB, table string, ts *TableSchema, row map[string]any) error {
	var cols []string
	var placeholders []string
	var values []any
	if r, ok := row["row"]; ok {
		if rr, ok := r.(map[string]any); ok {
			for _, col := range ts.Columns {
				if v, ok := rr[col.Name]; ok {
					cols = append(cols, col.Name)
					placeholders = append(placeholders, "?")
					values = append(values, v)
				}
			}
		}
	}

	if len(cols) == 0 {
		return fmt.Errorf("no columns to upsert")
	}

	query := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := db.Exec(query, values...)
	return err
}

// DeleteRow removes a row by primary key.
func (e *Editor) DeleteRow(table string, key map[string]any) error {
	ts, ok := e.Schema[table]
	if !ok {
		return fmt.Errorf("table not found: %s", table)
	}

	db, err := e.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	var conditions []string
	var values []any
	for _, pk := range ts.PrimaryKey {
		if v, ok := key[pk]; ok {
			conditions = append(conditions, fmt.Sprintf("%s = ?", pk))
			values = append(values, v)
		}
	}
	if len(conditions) == 0 {
		return fmt.Errorf("no primary key values provided")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, strings.Join(conditions, " AND "))
	_, err = db.Exec(query, values...)
	return err
}

// LookupOptions returns column value options for dropdown UIs.
func (e *Editor) LookupOptions(column, userID string) ([]map[string]any, error) {
	// This will be filled by the resolver integration
	return nil, nil
}

// --- Helpers ---

func isUserScoped(ts *TableSchema) bool {
	for _, col := range ts.Columns {
		if col.Name == "user_id" {
			return true
		}
	}
	return false
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}

func buildTableGroups(schema map[string]*TableSchema) []TableGroup {
	userTables := []string{}
	otherTables := []string{}

	for name, ts := range schema {
		if isUserScoped(ts) {
			userTables = append(userTables, name)
		} else {
			otherTables = append(otherTables, name)
		}
	}
	sort.Strings(userTables)
	sort.Strings(otherTables)

	// Group user tables by category
	categories := map[string][]string{}
	categoryOrder := []string{}
	for _, table := range userTables {
		cat := tableCategory(table)
		if _, ok := categories[cat]; !ok {
			categoryOrder = append(categoryOrder, cat)
		}
		categories[cat] = append(categories[cat], table)
	}

	var groups []TableGroup
	for _, cat := range categoryOrder {
		groups = append(groups, TableGroup{
			Key:    strings.ReplaceAll(strings.ToLower(cat), " ", "_"),
			Label:  cat,
			Tables: categories[cat],
		})
	}
	if len(otherTables) > 0 {
		groups = append(groups, TableGroup{
			Key:    "system",
			Label:  "System",
			Tables: otherTables,
		})
	}
	return groups
}

func tableCategory(table string) string {
	prefixes := map[string]string{
		"user_deck":              "Decks",
		"user_costume":           "Costumes",
		"user_weapon":            "Weapons",
		"user_companion":         "Companions",
		"user_character":         "Characters",
		"user_quest":             "Quests",
		"user_mission":           "Missions",
		"user_gacha":             "Gacha",
		"user_shop":              "Shop",
		"user_explore":           "Explore",
		"user_consumable":        "Inventory",
		"user_material":          "Inventory",
		"user_important":         "Inventory",
		"user_gift":              "Inventory",
		"user_parts":             "Parts",
		"user_thought":           "Thoughts",
		"user_big_hunt":          "Big Hunt",
		"user_pvp":               "PvP",
		"user_login":             "Login",
		"user_dokan":             "Dokan",
		"user_tutorial":          "Tutorial",
		"user_navi":              "Navigation",
		"user_limited_open":      "Limited",
		"user_gimmick":           "Gimmick",
		"user_portal":            "Portal Cage",
		"user_cage":              "Portal Cage",
	}
	for prefix, cat := range prefixes {
		if strings.HasPrefix(table, prefix) {
			return cat
		}
	}
	if strings.HasPrefix(table, "user_") {
		return "Other User Data"
	}
	return "System"
}
