package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Inspector inspects SQLite tables, pragma table_info, and foreign keys.
type Inspector struct {
	db *sql.DB
}

// NewInspector creates a new SQLite inspector.
func NewInspector(db *sql.DB) *Inspector {
	return &Inspector{db: db}
}

// InspectSchema discovers all user tables and constructs the schema definition.
func (ins *Inspector) InspectSchema(ctx context.Context, schemaName string) (*discovery.Schema, error) {
	if schemaName == "" {
		schemaName = "main"
	}

	query := `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;`
	rows, err := ins.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing sqlite tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}

	schema := &discovery.Schema{
		Name:   schemaName,
		Tables: make(map[string]*discovery.Table),
	}

	for _, name := range tableNames {
		tbl, err := ins.InspectTable(ctx, schemaName, name)
		if err != nil {
			return nil, fmt.Errorf("inspecting sqlite table %s: %w", name, err)
		}
		schema.Tables[name] = tbl
	}

	return schema, nil
}

// InspectTable retrieves columns, data types, primary keys, and foreign keys for a specific table.
func (ins *Inspector) InspectTable(ctx context.Context, schemaName, tableName string) (*discovery.Table, error) {
	table := &discovery.Table{
		Schema:     schemaName,
		Name:       tableName,
		Columns:    make(map[string]*discovery.Column),
		UniqueKeys: make(map[string]*discovery.UniqueKey),
	}

	// PRAGMA table_info(tableName)
	// cid, name, type, notnull, dflt_value, pk
	infoQuery := fmt.Sprintf("PRAGMA table_info(%q);", tableName)
	rows, err := ins.db.QueryContext(ctx, infoQuery)
	if err != nil {
		return nil, fmt.Errorf("inspecting columns for %s: %w", tableName, err)
	}
	defer rows.Close()

	var pkCols []string
	var ordinal int

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltVal sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltVal, &pk); err != nil {
			return nil, err
		}

		ordinal++
		col := &discovery.Column{
			Name:            name,
			OrdinalPosition: ordinal,
			DataType:        normalizeSQLiteType(colType),
			RawType:         colType,
			IsNullable:      notNull == 0,
		}

		if dfltVal.Valid {
			v := dfltVal.String
			col.DefaultValue = &v
		}

		if pk > 0 {
			col.IsPrimaryKey = true
			pkCols = append(pkCols, name)
		}

		table.Columns[name] = col
		table.ColumnOrder = append(table.ColumnOrder, name)
	}

	if len(pkCols) > 0 {
		table.PrimaryKey = &discovery.PrimaryKey{
			Name:    fmt.Sprintf("pk_%s", tableName),
			Columns: pkCols,
		}
	}

	// Foreign keys via PRAGMA foreign_key_list(tableName)
	// id, seq, table, from, to, on_update, on_delete, match
	fkQuery := fmt.Sprintf("PRAGMA foreign_key_list(%q);", tableName)
	fkRows, err := ins.db.QueryContext(ctx, fkQuery)
	if err == nil {
		defer fkRows.Close()
		type fkEntry struct {
			id       int
			seq      int
			refTable string
			fromCol  string
			toCol    string
			onUpdate string
			onDelete string
			match    string
		}

		fkMap := make(map[int][]*fkEntry)
		for fkRows.Next() {
			var e fkEntry
			if err := fkRows.Scan(&e.id, &e.seq, &e.refTable, &e.fromCol, &e.toCol, &e.onUpdate, &e.onDelete, &e.match); err == nil {
				fkMap[e.id] = append(fkMap[e.id], &e)
				if col, ok := table.Columns[e.fromCol]; ok {
					col.IsForeignKey = true
				}
			}
		}

		for id, entries := range fkMap {
			var fromCols []string
			var toCols []string
			refTable := ""
			onDel := ""
			onUpd := ""
			for _, e := range entries {
				refTable = e.refTable
				fromCols = append(fromCols, e.fromCol)
				toCols = append(toCols, e.toCol)
				onDel = e.onDelete
				onUpd = e.onUpdate
			}
			table.ForeignKeys = append(table.ForeignKeys, &discovery.ForeignKey{
				Name:              fmt.Sprintf("fk_%s_%d", tableName, id),
				Columns:           fromCols,
				ReferencedTable:   refTable,
				ReferencedSchema:  schemaName,
				ReferencedColumns: toCols,
				OnDelete:          onDel,
				OnUpdate:          onUpd,
			})
		}
	}

	// Approximate row count
	var count int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %q;", tableName)
	_ = ins.db.QueryRowContext(ctx, countQuery).Scan(&count)
	table.RowCount = count

	return table, nil
}

// SampleColumnValues retrieves a sample of non-null column values for PII heuristics.
func (ins *Inspector) SampleColumnValues(ctx context.Context, schemaName, tableName, columnName string, limit int) ([]any, error) {
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf("SELECT %q FROM %q WHERE %q IS NOT NULL LIMIT %d;", columnName, tableName, columnName, limit)
	rows, err := ins.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []any
	for rows.Next() {
		var val any
		if err := rows.Scan(&val); err == nil && val != nil {
			if b, ok := val.([]byte); ok {
				samples = append(samples, string(b))
			} else {
				samples = append(samples, val)
			}
		}
	}
	return samples, nil
}

func normalizeSQLiteType(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case strings.Contains(raw, "INT"):
		return "integer"
	case strings.Contains(raw, "CHAR"), strings.Contains(raw, "CLOB"), strings.Contains(raw, "TEXT"):
		return "text"
	case strings.Contains(raw, "BLOB"):
		return "blob"
	case strings.Contains(raw, "REAL"), strings.Contains(raw, "FLOA"), strings.Contains(raw, "DOUB"):
		return "real"
	case strings.Contains(raw, "NUM"), strings.Contains(raw, "DEC"), strings.Contains(raw, "DATE"):
		return "numeric"
	default:
		return "text"
	}
}
