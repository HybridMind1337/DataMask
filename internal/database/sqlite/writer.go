package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Writer writes batched rows and manages schema structure in SQLite.
type Writer struct {
	db       *sql.DB
	readOnly bool
}

// NewWriter instantiates an SQLite writer.
func NewWriter(db *sql.DB, readOnly bool) *Writer {
	return &Writer{db: db, readOnly: readOnly}
}

// WriteBatch inserts a batch of rows within a single SQLite transaction.
func (w *Writer) WriteBatch(ctx context.Context, table *discovery.Table, batch database.Batch) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	if len(batch.Rows) == 0 {
		return nil
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning sqlite transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	quotedCols := make([]string, len(batch.Columns))
	placeholders := make([]string, len(batch.Columns))
	for i, c := range batch.Columns {
		quotedCols[i] = fmt.Sprintf("%q", c)
		placeholders[i] = "?"
	}

	insertSQL := fmt.Sprintf(
		"INSERT INTO %q (%s) VALUES (%s);",
		table.Name,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)

	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("preparing sqlite insert statement: %w", err)
	}
	defer stmt.Close()

	for _, row := range batch.Rows {
		vals := make([]any, len(batch.Columns))
		for i, colName := range batch.Columns {
			vals[i] = row.Values[colName]
		}
		if _, err := stmt.ExecContext(ctx, vals...); err != nil {
			return fmt.Errorf("executing sqlite insert for %s: %w", table.Name, err)
		}
	}

	return tx.Commit()
}

// TruncateTable deletes all rows from the SQLite table.
func (w *Writer) TruncateTable(ctx context.Context, table *discovery.Table) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}
	_, err := w.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %q;", table.Name))
	return err
}

// CreateTableFromSource translates source table metadata to an SQLite table.
func (w *Writer) CreateTableFromSource(ctx context.Context, table *discovery.Table) error {
	if w.readOnly {
		return database.ErrReadOnlyViolation
	}

	var colDefs []string
	for _, colName := range table.ColumnOrder {
		col := table.Columns[colName]
		nullability := ""
		if !col.IsNullable {
			nullability = "NOT NULL"
		}
		sqlType := mapToSQLiteType(col.DataType)
		colDefs = append(colDefs, fmt.Sprintf("  %q %s %s", col.Name, sqlType, nullability))
	}

	if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0 {
		pkCols := make([]string, len(table.PrimaryKey.Columns))
		for i, c := range table.PrimaryKey.Columns {
			pkCols[i] = fmt.Sprintf("%q", c)
		}
		colDefs = append(colDefs, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	for _, fk := range table.ForeignKeys {
		var fromCols []string
		var toCols []string
		for _, c := range fk.Columns {
			fromCols = append(fromCols, fmt.Sprintf("%q", c))
		}
		for _, c := range fk.ReferencedColumns {
			toCols = append(toCols, fmt.Sprintf("%q", c))
		}
		fkDef := fmt.Sprintf("  FOREIGN KEY (%s) REFERENCES %q (%s)",
			strings.Join(fromCols, ", "),
			fk.ReferencedTable,
			strings.Join(toCols, ", "))
		if fk.OnDelete != "" {
			fkDef += fmt.Sprintf(" ON DELETE %s", fk.OnDelete)
		}
		if fk.OnUpdate != "" {
			fkDef += fmt.Sprintf(" ON UPDATE %s", fk.OnUpdate)
		}
		colDefs = append(colDefs, fkDef)
	}

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %q (\n%s\n);",
		table.Name,
		strings.Join(colDefs, ",\n"))

	_, err := w.db.ExecContext(ctx, createSQL)
	return err
}

func mapToSQLiteType(t string) string {
	t = strings.ToLower(t)
	switch {
	case strings.Contains(t, "int"), strings.Contains(t, "bool"), strings.Contains(t, "serial"):
		return "INTEGER"
	case strings.Contains(t, "char"), strings.Contains(t, "text"), strings.Contains(t, "uuid"), strings.Contains(t, "json"):
		return "TEXT"
	case strings.Contains(t, "float"), strings.Contains(t, "double"), strings.Contains(t, "real"):
		return "REAL"
	case strings.Contains(t, "blob"), strings.Contains(t, "bytea"):
		return "BLOB"
	case strings.Contains(t, "num"), strings.Contains(t, "dec"), strings.Contains(t, "date"), strings.Contains(t, "time"):
		return "NUMERIC"
	default:
		return "TEXT"
	}
}
