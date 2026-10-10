package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Writer writes transformed batches into PostgreSQL using high-speed pgx.CopyFrom.
type Writer struct {
	pool     *pgxpool.Pool
	readOnly bool
}

// NewWriter creates a writer.
func NewWriter(pool *pgxpool.Pool, readOnly bool) *Writer {
	return &Writer{pool: pool, readOnly: readOnly}
}

// WriteBatch performs bulk insertion of a batch into the target table.
func (w *Writer) WriteBatch(ctx context.Context, table *discovery.Table, batch database.Batch) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	if len(batch.Rows) == 0 {
		return nil
	}

	// Prepare data matrix for pgx.CopyFrom
	matrix := make([][]any, len(batch.Rows))
	for rIdx, r := range batch.Rows {
		rowVals := make([]any, len(batch.Columns))
		for cIdx, colName := range batch.Columns {
			rowVals[cIdx] = r.Values[colName]
		}
		matrix[rIdx] = rowVals
	}

	targetTable := pgx.Identifier{table.Name}
	if table.Schema != "" && table.Schema != "public" {
		targetTable = pgx.Identifier{table.Schema, table.Name}
	}

	_, err := w.pool.CopyFrom(
		ctx,
		targetTable,
		batch.Columns,
		pgx.CopyFromRows(matrix),
	)
	if err != nil {
		// Fallback to parameterized chunked multi-insert if CopyFrom is rejected due to triggers/generated columns
		return w.insertFallback(ctx, table, batch)
	}

	return nil
}

func (w *Writer) insertFallback(ctx context.Context, table *discovery.Table, batch database.Batch) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	quotedCols := make([]string, len(batch.Columns))
	for i, c := range batch.Columns {
		quotedCols[i] = fmt.Sprintf("%q", c)
	}
	colsList := strings.Join(quotedCols, ", ")

	numCols := len(batch.Columns)
	if numCols == 0 {
		return nil
	}
	maxRowsPerStmt := 65535 / numCols
	if maxRowsPerStmt <= 0 || maxRowsPerStmt > 1000 {
		maxRowsPerStmt = 1000
	}

	for i := 0; i < len(batch.Rows); i += maxRowsPerStmt {
		end := i + maxRowsPerStmt
		if end > len(batch.Rows) {
			end = len(batch.Rows)
		}
		chunk := batch.Rows[i:end]

		var valTuples []string
		var args []any
		paramIdx := 1

		for _, row := range chunk {
			placeholders := make([]string, numCols)
			for cIdx, c := range batch.Columns {
				placeholders[cIdx] = fmt.Sprintf("$%d", paramIdx)
				args = append(args, row.Values[c])
				paramIdx++
			}
			valTuples = append(valTuples, fmt.Sprintf("(%s)", strings.Join(placeholders, ", ")))
		}

		sql := fmt.Sprintf("INSERT INTO %q.%q (%s) VALUES %s;", table.Schema, table.Name, colsList, strings.Join(valTuples, ", "))
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// TruncateTable cleans destination table before loading.
func (w *Writer) TruncateTable(ctx context.Context, table *discovery.Table) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	query := fmt.Sprintf("TRUNCATE TABLE %q.%q CASCADE;", table.Schema, table.Name)
	_, err := w.pool.Exec(ctx, query)
	return err
}

// CreateTableFromSource clones the table structure if target table does not exist.
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
		colDefs = append(colDefs, fmt.Sprintf("%q %s %s", col.Name, dType, nullability))
	}

	if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0 {
		pkCols := make([]string, len(table.PrimaryKey.Columns))
		for i, c := range table.PrimaryKey.Columns {
			pkCols[i] = fmt.Sprintf("%q", c)
		}
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %q.%q (\n  %s\n);", table.Schema, table.Name, strings.Join(colDefs, ",\n  "))
	_, err := w.pool.Exec(ctx, sql)
	return err
}
