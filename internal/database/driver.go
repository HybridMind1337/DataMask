package database

import (
	"context"
	"fmt"
	"io"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Driver abstracts database engine-specific operations.
type Driver interface {
	io.Closer
	Type() string
	Ping(ctx context.Context) error
	Inspector() SchemaInspector
	Reader() RowReader
	Writer() RowWriter
	Constraints() ConstraintManager
	ExecDirect(ctx context.Context, query string) error
}

// SchemaInspector discovers tables, columns, data types, and constraints.
type SchemaInspector interface {
	InspectSchema(ctx context.Context, schemaName string) (*discovery.Schema, error)
	InspectTable(ctx context.Context, schemaName, tableName string) (*discovery.Table, error)
	SampleColumnValues(ctx context.Context, schemaName, tableName, columnName string, limit int) ([]any, error)
}

// Row represents a single database row mapped by column names.
type Row struct {
	Values map[string]any
}

// Batch represents a chunk of streamed rows for a table.
type Batch struct {
	TableName string
	Columns   []string
	Rows      []Row
}

// RowReader streams rows in bounded batches.
type RowReader interface {
	ReadTableStream(ctx context.Context, table *discovery.Table, batchSize int) (<-chan Batch, <-chan error)
}

// RowWriter handles chunked writing into the target database.
type RowWriter interface {
	WriteBatch(ctx context.Context, table *discovery.Table, batch Batch) error
	TruncateTable(ctx context.Context, table *discovery.Table) error
	CreateTableFromSource(ctx context.Context, table *discovery.Table) error
}

// ConstraintManager controls constraint toggling during loading.
type ConstraintManager interface {
	DisableConstraints(ctx context.Context, table *discovery.Table) error
	EnableConstraints(ctx context.Context, table *discovery.Table) error
	DisableAllForeignKeys(ctx context.Context, schema string) error
	EnableAllForeignKeys(ctx context.Context, schema string) error
}

// ErrReadOnlyViolation is returned when a write operation is attempted against a read-only connection.
var ErrReadOnlyViolation = fmt.Errorf("read-only violation: operation rejected on protected source database")
