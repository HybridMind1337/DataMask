package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Inspector discovers MySQL / MariaDB schema structures.
type Inspector struct {
	db           *sql.DB
	databaseName string
}

// NewInspector creates a MySQL inspector.
func NewInspector(db *sql.DB, databaseName string) *Inspector {
	return &Inspector{db: db, databaseName: databaseName}
}

// InspectSchema discovers all tables, columns, keys, and foreign keys in MySQL.
func (ins *Inspector) InspectSchema(ctx context.Context, databaseName string) (*discovery.Schema, error) {
	if databaseName == "" {
		databaseName = ins.databaseName
	}

	schema := &discovery.Schema{
		Name:   databaseName,
		Tables: make(map[string]*discovery.Table),
	}

	// 1. Tables
	tQuery := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name;
	`
	rows, err := ins.db.QueryContext(ctx, tQuery, databaseName)
	if err != nil {
		return nil, fmt.Errorf("querying mysql tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tName string
		if err := rows.Scan(&tName); err != nil {
			return nil, err
		}
		schema.Tables[tName] = &discovery.Table{
			Schema:      databaseName,
			Name:        tName,
			Columns:     make(map[string]*discovery.Column),
			ColumnOrder: make([]string, 0),
			UniqueKeys:  make(map[string]*discovery.UniqueKey),
		}
	}

	// 2. Columns
	cQuery := `
		SELECT table_name, column_name, ordinal_position, data_type, column_type, is_nullable, column_default, character_maximum_length, numeric_precision, numeric_scale
		FROM information_schema.columns
		WHERE table_schema = ?
		ORDER BY table_name, ordinal_position;
	`
	cRows, err := ins.db.QueryContext(ctx, cQuery, databaseName)
	if err != nil {
		return nil, fmt.Errorf("querying mysql columns: %w", err)
	}
	defer cRows.Close()

	for cRows.Next() {
		var tName, cName, dType, colType, isNullStr string
		var ordPos int
		var cDefault sql.NullString
		var maxLen, numPrec, numScale sql.NullInt64

		if err := cRows.Scan(&tName, &cName, &ordPos, &dType, &colType, &isNullStr, &cDefault, &maxLen, &numPrec, &numScale); err != nil {
			return nil, err
		}

		table, exists := schema.Tables[tName]
		if !exists {
			continue
		}

		var defPtr *string
		if cDefault.Valid {
			defPtr = &cDefault.String
		}
		var maxLenPtr, numPrecPtr, numScalePtr *int
		if maxLen.Valid {
			v := int(maxLen.Int64)
			maxLenPtr = &v
		}
		if numPrec.Valid {
			v := int(numPrec.Int64)
			numPrecPtr = &v
		}
		if numScale.Valid {
			v := int(numScale.Int64)
			numScalePtr = &v
		}

		col := &discovery.Column{
			Name:             cName,
			OrdinalPosition:  ordPos,
			DataType:         dType,
			RawType:          colType,
			IsNullable:       strings.ToUpper(isNullStr) == "YES",
			DefaultValue:     defPtr,
			MaxLength:        maxLenPtr,
			NumericPrecision: numPrecPtr,
			NumericScale:     numScalePtr,
		}
		table.Columns[cName] = col
		table.ColumnOrder = append(table.ColumnOrder, cName)
	}

	// 3. Primary & Unique Keys
	kQuery := `
		SELECT table_name, constraint_name, column_name, ordinal_position
		FROM information_schema.key_column_usage
		WHERE table_schema = ? AND referenced_table_name IS NULL
		ORDER BY table_name, constraint_name, ordinal_position;
	`
	kRows, err := ins.db.QueryContext(ctx, kQuery, databaseName)
	if err == nil {
		defer kRows.Close()
		for kRows.Next() {
			var tName, cName, colName string
			var pos int
			if err := kRows.Scan(&tName, &cName, &colName, &pos); err == nil {
				if table, ok := schema.Tables[tName]; ok {
					if cName == "PRIMARY" {
						if table.PrimaryKey == nil {
							table.PrimaryKey = &discovery.PrimaryKey{Name: "PRIMARY", Columns: []string{}}
						}
						table.PrimaryKey.Columns = append(table.PrimaryKey.Columns, colName)
						if col, colOk := table.Columns[colName]; colOk {
							col.IsPrimaryKey = true
						}
					} else {
						if _, uqOk := table.UniqueKeys[cName]; !uqOk {
							table.UniqueKeys[cName] = &discovery.UniqueKey{Name: cName, Columns: []string{}}
						}
						table.UniqueKeys[cName].Columns = append(table.UniqueKeys[cName].Columns, colName)
						if col, colOk := table.Columns[colName]; colOk {
							col.IsUnique = true
						}
					}
				}
			}
		}
	}

	// 4. Foreign Keys
	fkQuery := `
		SELECT table_name, constraint_name, column_name, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ? AND referenced_table_name IS NOT NULL
		ORDER BY table_name, constraint_name, ordinal_position;
	`
	fkRows, err := ins.db.QueryContext(ctx, fkQuery, databaseName)
	if err == nil {
		defer fkRows.Close()
		for fkRows.Next() {
			var tName, cName, colName, refTable, refCol string
			if err := fkRows.Scan(&tName, &cName, &colName, &refTable, &refCol); err == nil {
				if table, ok := schema.Tables[tName]; ok {
					table.ForeignKeys = append(table.ForeignKeys, &discovery.ForeignKey{
						Name:              cName,
						Columns:           []string{colName},
						ReferencedTable:   refTable,
						ReferencedSchema:  databaseName,
						ReferencedColumns: []string{refCol},
					})
					if col, colOk := table.Columns[colName]; colOk {
						col.IsForeignKey = true
					}
				}
			}
		}
	}

	// 5. Row counts
	for _, table := range schema.Tables {
		var cnt int64
		_ = ins.db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM `%s`.`%s`;", databaseName, table.Name)).Scan(&cnt)
		table.RowCount = cnt
	}

	return schema, nil
}

// InspectTable inspects a single MySQL table.
func (ins *Inspector) InspectTable(ctx context.Context, databaseName, tableName string) (*discovery.Table, error) {
	schema, err := ins.InspectSchema(ctx, databaseName)
	if err != nil {
		return nil, err
	}
	t, ok := schema.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %q not found in mysql database %q", tableName, databaseName)
	}
	return t, nil
}

// SampleColumnValues retrieves a small non-destructive sample.
func (ins *Inspector) SampleColumnValues(ctx context.Context, databaseName, tableName, columnName string, limit int) ([]any, error) {
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf("SELECT `%s` FROM `%s`.`%s` WHERE `%s` IS NOT NULL LIMIT %d;", columnName, databaseName, tableName, columnName, limit)
	rows, err := ins.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []any
	for rows.Next() {
		var val any
		if err := rows.Scan(&val); err == nil {
			samples = append(samples, val)
		}
	}
	return samples, nil
}
