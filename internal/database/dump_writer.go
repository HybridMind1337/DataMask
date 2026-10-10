package database

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// DumpWriter streams sanitised SQL directly to an output file (supporting .sql and .sql.gz).
type DumpWriter struct {
	mu       sync.Mutex
	filePath string
	file     *os.File
	gzWriter *gzip.Writer
	writer   io.Writer
	driver   string // postgres or mysql syntax
}

// NewDumpWriter creates a writer that streams plain SQL or gzip-compressed SQL.
func NewDumpWriter(filePath string, driver string) (*DumpWriter, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating dump output file: %w", err)
	}

	dw := &DumpWriter{
		filePath: filePath,
		file:     f,
		driver:   strings.ToLower(driver),
	}

	if strings.HasSuffix(strings.ToLower(filePath), ".gz") {
		dw.gzWriter = gzip.NewWriter(f)
		dw.writer = dw.gzWriter
	} else {
		dw.writer = f
	}

	// Write standard SQL header
	header := fmt.Sprintf("-- DataMask Sanitised Database Export\n-- Driver: %s\n-- Generated for Development and Staging\n\n", driver)
	if _, err := io.WriteString(dw.writer, header); err != nil {
		_ = dw.Close()
		return nil, err
	}

	return dw, nil
}

// WriteBatch formats transformed rows as SQL INSERT statements and writes to the stream.
func (d *DumpWriter) WriteBatch(ctx context.Context, table *discovery.Table, batch Batch) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(batch.Rows) == 0 {
		return nil
	}

	var sb strings.Builder
	colList := strings.Join(batch.Columns, ", ")
	if d.driver == "postgres" || d.driver == "sqlite" {
		quotedCols := make([]string, len(batch.Columns))
		for i, c := range batch.Columns {
			quotedCols[i] = fmt.Sprintf("%q", c)
		}
		colList = strings.Join(quotedCols, ", ")
	} else {
		quotedCols := make([]string, len(batch.Columns))
		for i, c := range batch.Columns {
			quotedCols[i] = fmt.Sprintf("`%s`", c)
		}
		colList = strings.Join(quotedCols, ", ")
	}

	for _, row := range batch.Rows {
		vals := make([]string, len(batch.Columns))
		for i, colName := range batch.Columns {
			v := row.Values[colName]
			vals[i] = formatSQLValue(v)
		}
		if d.driver == "postgres" {
			sb.WriteString(fmt.Sprintf("INSERT INTO %q.%q (%s) VALUES (%s);\n",
				table.Schema, table.Name, colList, strings.Join(vals, ", ")))
		} else if d.driver == "sqlite" {
			sb.WriteString(fmt.Sprintf("INSERT INTO %q (%s) VALUES (%s);\n",
				table.Name, colList, strings.Join(vals, ", ")))
		} else {
			sb.WriteString(fmt.Sprintf("INSERT INTO `%s`.`%s` (%s) VALUES (%s);\n",
				table.Schema, table.Name, colList, strings.Join(vals, ", ")))
		}
	}

	_, err := io.WriteString(d.writer, sb.String())
	return err
}

func (d *DumpWriter) TruncateTable(ctx context.Context, table *discovery.Table) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	stmt := fmt.Sprintf("\n-- Table: %s\nTRUNCATE TABLE %s;\n", table.Name, table.Name)
	_, err := io.WriteString(d.writer, stmt)
	return err
}

func (d *DumpWriter) CreateTableFromSource(ctx context.Context, table *discovery.Table) error {
	d.mu.Lock()
	defer d.mu.Unlock()

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
		colDefs = append(colDefs, fmt.Sprintf("  %s %s %s", col.Name, dType, nullability))
	}

	if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0 {
		colDefs = append(colDefs, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(table.PrimaryKey.Columns, ", ")))
	}

	stmt := fmt.Sprintf("\nCREATE TABLE IF NOT EXISTS %s (\n%s\n);\n", table.Name, strings.Join(colDefs, ",\n"))
	_, err := io.WriteString(d.writer, stmt)
	return err
}

// Close flushes gzip buffer and closes file descriptor.
func (d *DumpWriter) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var firstErr error
	if d.gzWriter != nil {
		if err := d.gzWriter.Close(); err != nil {
			firstErr = err
		}
	}
	if d.file != nil {
		if err := d.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// formatSQLValue formats a Go value into a safe SQL literal.
func formatSQLValue(val any) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case string:
		escaped := strings.ReplaceAll(v, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case []byte:
		escaped := strings.ReplaceAll(string(v), "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// DumpDriver implements database.Driver wrapper around a DumpWriter.
type DumpDriver struct {
	dumpWriter *DumpWriter
}

func NewDumpDriver(dw *DumpWriter) *DumpDriver {
	return &DumpDriver{dumpWriter: dw}
}

func (d *DumpDriver) Type() string { return "dump" }
func (d *DumpDriver) Ping(ctx context.Context) error { return nil }
func (d *DumpDriver) Close() error { return d.dumpWriter.Close() }
func (d *DumpDriver) Inspector() SchemaInspector { return nil }
func (d *DumpDriver) Reader() RowReader { return nil }
func (d *DumpDriver) Writer() RowWriter { return d.dumpWriter }
func (d *DumpDriver) Constraints() ConstraintManager { return &noopConstraintManager{} }
func (d *DumpDriver) ExecDirect(ctx context.Context, query string) error { return nil }

type noopConstraintManager struct{}
func (n *noopConstraintManager) DisableConstraints(ctx context.Context, table *discovery.Table) error { return nil }
func (n *noopConstraintManager) EnableConstraints(ctx context.Context, table *discovery.Table) error { return nil }
func (n *noopConstraintManager) DisableAllForeignKeys(ctx context.Context, schema string) error { return nil }
func (n *noopConstraintManager) EnableAllForeignKeys(ctx context.Context, schema string) error { return nil }
