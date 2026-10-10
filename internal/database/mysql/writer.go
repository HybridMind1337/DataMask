package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Writer writes transformed batches into MySQL using bulk multi-row insert statements.
type Writer struct {
	db       *sql.DB
	readOnly bool
}

// NewWriter creates a MySQL writer.
func NewWriter(db *sql.DB, readOnly bool) *Writer {
	return &Writer{db: db, readOnly: readOnly}
}

// WriteBatch writes a batch using multi-row INSERT INTO table (cols) VALUES (...), (...).
func (w *Writer) WriteBatch(ctx context.Context, table *discovery.Table, batch database.Batch) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	if len(batch.Rows) == 0 {
		return nil
	}

	quotedCols := make([]string, len(batch.Columns))
	for i, c := range batch.Columns {
		quotedCols[i] = fmt.Sprintf("`%s`", c)
	}
	colsList := strings.Join(quotedCols, ", ")

	// MySQL limits max placeholders (usually up to 65535). Chunk rows if needed.
	maxRowsPerInsert := 1000
	if len(batch.Columns) > 0 {
		maxRowsPerInsert = 60000 / len(batch.Columns)
		if maxRowsPerInsert <= 0 {
			maxRowsPerInsert = 1
		}
	}

	for i := 0; i < len(batch.Rows); i += maxRowsPerInsert {
		end := i + maxRowsPerInsert
		if end > len(batch.Rows) {
			end = len(batch.Rows)
		}
		chunk := batch.Rows[i:end]

		var valueStrings []string
		var valueArgs []any

		for _, row := range chunk {
			placeholders := make([]string, len(batch.Columns))
			for colIdx, colName := range batch.Columns {
				placeholders[colIdx] = "?"
				valueArgs = append(valueArgs, row.Values[colName])
			}
			valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(placeholders, ", ")))
		}

		stmt := fmt.Sprintf("INSERT INTO `%s`.`%s` (%s) VALUES %s;",
			table.Schema, table.Name, colsList, strings.Join(valueStrings, ", "))

		if _, err := w.db.ExecContext(ctx, stmt, valueArgs...); err != nil {
			return fmt.Errorf("bulk inserting into mysql %s: %w", table.Name, err)
		}
	}

	return nil
}

// TruncateTable cleans the destination table.
func (w *Writer) TruncateTable(ctx context.Context, table *discovery.Table) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	query := fmt.Sprintf("TRUNCATE TABLE `%s`.`%s`;", table.Schema, table.Name)
	_, err := w.db.ExecContext(ctx, query)
	return err
}

// CreateTableFromSource clones the basic table structure.
func (w *Writer) CreateTableFromSource(ctx context.Context, table *discovery.Table) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	var colDefs []string
	for _, colName := range table.ColumnOrder {
		col := table.Columns[colName]
		nullability := "NULL"
		if !col.IsNullable {
			nullability = "NOT NULL"
		}
		dType := col.DataType
		if col.RawType != "" {
			dType = col.RawType
		}
		colDefs = append(colDefs, fmt.Sprintf("`%s` %s %s", col.Name, dType, nullability))
	}

	if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0 {
		pkCols := make([]string, len(table.PrimaryKey.Columns))
		for i, c := range table.PrimaryKey.Columns {
			pkCols[i] = fmt.Sprintf("`%s`", c)
		}
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` (\n  %s\n);", table.Schema, table.Name, strings.Join(colDefs, ",\n  "))
	_, err := w.db.ExecContext(ctx, sql)
	return err
}
