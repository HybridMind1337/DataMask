package discovery

import "fmt"

// Schema represents the complete structural metadata of an inspected database.
type Schema struct {
	Name   string            `json:"name" yaml:"name"`
	Tables map[string]*Table `json:"tables" yaml:"tables"`
}

// Table represents a database table including columns, keys, and constraints.
type Table struct {
	Schema      string                 `json:"schema" yaml:"schema"`
	Name        string                 `json:"name" yaml:"name"`
	Columns     map[string]*Column     `json:"columns" yaml:"columns"`
	ColumnOrder []string               `json:"column_order" yaml:"column_order"`
	PrimaryKey  *PrimaryKey            `json:"primary_key,omitempty" yaml:"primary_key,omitempty"`
	ForeignKeys []*ForeignKey          `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty"`
	UniqueKeys  map[string]*UniqueKey  `json:"unique_keys,omitempty" yaml:"unique_keys,omitempty"`
	RowCount    int64                  `json:"row_count" yaml:"row_count"`
}

// Column represents metadata for a table column.
type Column struct {
	Name            string `json:"name" yaml:"name"`
	OrdinalPosition int    `json:"ordinal_position" yaml:"ordinal_position"`
	DataType        string `json:"data_type" yaml:"data_type"`
	RawType         string `json:"raw_type" yaml:"raw_type"`
	IsNullable      bool   `json:"is_nullable" yaml:"is_nullable"`
	DefaultValue    *string `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	MaxLength       *int   `json:"max_length,omitempty" yaml:"max_length,omitempty"`
	NumericPrecision *int  `json:"numeric_precision,omitempty" yaml:"numeric_precision,omitempty"`
	NumericScale    *int   `json:"numeric_scale,omitempty" yaml:"numeric_scale,omitempty"`
	IsPrimaryKey    bool   `json:"is_primary_key" yaml:"is_primary_key"`
	IsForeignKey    bool   `json:"is_foreign_key" yaml:"is_foreign_key"`
	IsUnique        bool   `json:"is_unique" yaml:"is_unique"`
}

// PrimaryKey captures table primary key constraint columns.
type PrimaryKey struct {
	Name    string   `json:"name" yaml:"name"`
	Columns []string `json:"columns" yaml:"columns"`
}

// ForeignKey defines an outbound referential relationship.
type ForeignKey struct {
	Name             string   `json:"name" yaml:"name"`
	Columns          []string `json:"columns" yaml:"columns"`
	ReferencedTable  string   `json:"referenced_table" yaml:"referenced_table"`
	ReferencedSchema string   `json:"referenced_schema" yaml:"referenced_schema"`
	ReferencedColumns []string `json:"referenced_columns" yaml:"referenced_columns"`
	OnDelete         string   `json:"on_delete,omitempty" yaml:"on_delete,omitempty"`
	OnUpdate         string   `json:"on_update,omitempty" yaml:"on_update,omitempty"`
}

// UniqueKey captures a unique constraint or unique index.
type UniqueKey struct {
	Name    string   `json:"name" yaml:"name"`
	Columns []string `json:"columns" yaml:"columns"`
}

// GetColumn returns the Column definition by name safely.
func (t *Table) GetColumn(colName string) (*Column, bool) {
	if t.Columns == nil {
		return nil, false
	}
	col, ok := t.Columns[colName]
	return col, ok
}

// FullName returns schema.table or just table name if schema is public or empty.
func (t *Table) FullName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return fmt.Sprintf("%s.%s", t.Schema, t.Name)
}
