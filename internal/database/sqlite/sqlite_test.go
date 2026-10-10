package sqlite

import (
	"context"
	"os"
	"testing"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

func TestSQLiteDriver(t *testing.T) {
	ctx := context.Background()
	tmpFile, err := os.CreateTemp("", "datamask_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	cfg := config.DatabaseConfig{
		Driver:   "sqlite",
		Database: dbPath,
	}

	// 1. Test ReadWrite Driver
	drv, err := NewDriver(ctx, cfg, false)
	if err != nil {
		t.Fatalf("NewDriver failed: %v", err)
	}
	defer drv.Close()

	if drv.Type() != "sqlite" {
		t.Errorf("expected type sqlite, got %s", drv.Type())
	}

	if err := drv.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}

	// 2. Create Table
	tbl := &discovery.Table{
		Name: "users",
		ColumnOrder: []string{"id", "email", "name"},
		Columns: map[string]*discovery.Column{
			"id":    {Name: "id", DataType: "integer", IsPrimaryKey: true},
			"email": {Name: "email", DataType: "text"},
			"name":  {Name: "name", DataType: "text"},
		},
		PrimaryKey: &discovery.PrimaryKey{
			Name:    "pk_users",
			Columns: []string{"id"},
		},
	}

	if err := drv.Writer().CreateTableFromSource(ctx, tbl); err != nil {
		t.Fatalf("CreateTableFromSource failed: %v", err)
	}

	// 3. Insert Batch
	batch := database.Batch{
		TableName: "users",
		Columns:   []string{"id", "email", "name"},
		Rows: []database.Row{
			{Values: map[string]any{"id": int64(1), "email": "alice@company.com", "name": "Alice"}},
			{Values: map[string]any{"id": int64(2), "email": "bob@company.com", "name": "Bob"}},
		},
	}
	if err := drv.Writer().WriteBatch(ctx, tbl, batch); err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}

	// 4. Test Inspector
	schema, err := drv.Inspector().InspectSchema(ctx, "main")
	if err != nil {
		t.Fatalf("InspectSchema failed: %v", err)
	}
	if _, exists := schema.Tables["users"]; !exists {
		t.Fatalf("users table missing from inspected schema")
	}

	// 5. Test Keyset Streaming Reader
	batchChan, errChan := drv.Reader().ReadTableStream(ctx, tbl, 1)
	var streamedRows int
	for b := range batchChan {
		streamedRows += len(b.Rows)
	}
	if err := <-errChan; err != nil {
		t.Fatalf("ReadTableStream error: %v", err)
	}
	if streamedRows != 2 {
		t.Errorf("expected 2 streamed rows, got %d", streamedRows)
	}

	// 6. Test Read-Only Driver
	roDrv, err := NewDriver(ctx, cfg, true)
	if err != nil {
		t.Fatalf("NewDriver read-only failed: %v", err)
	}
	defer roDrv.Close()

	if err := roDrv.Writer().TruncateTable(ctx, tbl); err != database.ErrReadOnlyViolation {
		t.Errorf("expected ErrReadOnlyViolation, got %v", err)
	}
}
