package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Inspector inspects PostgreSQL schemas.
type Inspector struct {
	pool       *pgxpool.Pool
	schemaName string
}

// NewInspector creates a new PostgreSQL inspector.
func NewInspector(pool *pgxpool.Pool, schemaName string) *Inspector {
	if schemaName == "" {
		schemaName = "public"
	}
	return &Inspector{pool: pool, schemaName: schemaName}
}

// InspectSchema discovers all tables, columns, PKs, FKs, and unique constraints.
func (ins *Inspector) InspectSchema(ctx context.Context, schemaName string) (*discovery.Schema, error) {
	if schemaName == "" {
		schemaName = ins.schemaName
	}

	schema := &discovery.Schema{
		Name:   schemaName,
		Tables: make(map[string]*discovery.Table),
	}

	// 1. Get Tables
	tableQuery := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		ORDER BY table_name;
	`
	rows, err := ins.pool.Query(ctx, tableQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("querying postgres tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tName string
		if err := rows.Scan(&tName); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, tName)
		schema.Tables[tName] = &discovery.Table{
			Schema:      schemaName,
			Name:        tName,
			Columns:     make(map[string]*discovery.Column),
			ColumnOrder: make([]string, 0),
			UniqueKeys:  make(map[string]*discovery.UniqueKey),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. Discover Columns for each table
	colQuery := `
		SELECT table_name, column_name, ordinal_position, data_type, udt_name, is_nullable, column_default, character_maximum_length, numeric_precision, numeric_scale
		FROM information_schema.columns
		WHERE table_schema = $1
		ORDER BY table_name, ordinal_position;
	`
	cRows, err := ins.pool.Query(ctx, colQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("querying postgres columns: %w", err)
	}
	defer cRows.Close()

	for cRows.Next() {
		var tName, cName, dType, udtName, isNullStr string
		var ordPos int
		var cDefault *string
		var maxLen, numPrec, numScale *int

		if err := cRows.Scan(&tName, &cName, &ordPos, &dType, &udtName, &isNullStr, &cDefault, &maxLen, &numPrec, &numScale); err != nil {
			return nil, err
		}

		table, exists := schema.Tables[tName]
		if !exists {
			continue
		}

		col := &discovery.Column{
			Name:             cName,
			OrdinalPosition:  ordPos,
			DataType:         dType,
			RawType:          udtName,
			IsNullable:       strings.ToUpper(isNullStr) == "YES",
			DefaultValue:     cDefault,
			MaxLength:        maxLen,
			NumericPrecision: numPrec,
			NumericScale:     numScale,
		}
		table.Columns[cName] = col
		table.ColumnOrder = append(table.ColumnOrder, cName)
	}

	// 3. Discover Primary Keys
	pkQuery := `
		SELECT tc.table_name, tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = $1 AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY tc.table_name, kcu.ordinal_position;
	`
	pkRows, err := ins.pool.Query(ctx, pkQuery, schemaName)
	if err == nil {
		defer pkRows.Close()
		for pkRows.Next() {
			var tName, cName, colName string
			if err := pkRows.Scan(&tName, &cName, &colName); err == nil {
				if table, ok := schema.Tables[tName]; ok {
					if table.PrimaryKey == nil {
						table.PrimaryKey = &discovery.PrimaryKey{Name: cName, Columns: []string{}}
					}
					table.PrimaryKey.Columns = append(table.PrimaryKey.Columns, colName)
					if col, colOk := table.Columns[colName]; colOk {
						col.IsPrimaryKey = true
					}
				}
			}
		}
	}

	// 4. Discover Foreign Keys
	fkQuery := `
		SELECT
			tc.table_name,
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.table_schema AS foreign_table_schema,
			ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.table_schema = $1 AND tc.constraint_type = 'FOREIGN KEY';
	`
	fkRows, err := ins.pool.Query(ctx, fkQuery, schemaName)
	if err == nil {
		defer fkRows.Close()
		for fkRows.Next() {
			var tName, cName, colName, fTable, fSchema, fCol string
			if err := fkRows.Scan(&tName, &cName, &colName, &fTable, &fSchema, &fCol); err == nil {
				if table, ok := schema.Tables[tName]; ok {
					table.ForeignKeys = append(table.ForeignKeys, &discovery.ForeignKey{
						Name:              cName,
						Columns:           []string{colName},
						ReferencedTable:   fTable,
						ReferencedSchema:  fSchema,
						ReferencedColumns: []string{fCol},
					})
					if col, colOk := table.Columns[colName]; colOk {
						col.IsForeignKey = true
					}
				}
			}
		}
	}

	// 5. Discover Unique Constraints
	uqQuery := `
		SELECT tc.table_name, tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = $1 AND tc.constraint_type = 'UNIQUE'
		ORDER BY tc.table_name, kcu.ordinal_position;
	`
	uqRows, err := ins.pool.Query(ctx, uqQuery, schemaName)
	if err == nil {
		defer uqRows.Close()
		for uqRows.Next() {
			var tName, cName, colName string
			if err := uqRows.Scan(&tName, &cName, &colName); err == nil {
				if table, ok := schema.Tables[tName]; ok {
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

	// 6. Estimate / count rows for progress bars
	for _, table := range schema.Tables {
		var cnt int64
		_ = ins.pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %q.%q;", schemaName, table.Name)).Scan(&cnt)
		table.RowCount = cnt
	}

	return schema, nil
}

// InspectTable inspects a single table's metadata.
func (ins *Inspector) InspectTable(ctx context.Context, schemaName, tableName string) (*discovery.Table, error) {
	schema, err := ins.InspectSchema(ctx, schemaName)
	if err != nil {
		return nil, err
	}
	table, ok := schema.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %q not found in schema %q", tableName, schemaName)
	}
	return table, nil
}

// SampleColumnValues retrieves a small sample of values for pattern detection.
func (ins *Inspector) SampleColumnValues(ctx context.Context, schemaName, tableName, columnName string, limit int) ([]any, error) {
	if limit <= 0 {
		limit = 50
	}
	if schemaName == "" {
		schemaName = ins.schemaName
	}
	query := fmt.Sprintf("SELECT %q FROM %q.%q WHERE %q IS NOT NULL LIMIT %d;", columnName, schemaName, tableName, columnName, limit)
	rows, err := ins.pool.Query(ctx, query)
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
