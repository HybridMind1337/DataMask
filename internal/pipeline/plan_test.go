package pipeline_test

import (
	"testing"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/discovery"
	"github.com/HybridMind1337/DataMask/internal/pipeline"
)

func TestCompilePlan(t *testing.T) {
	schema := &discovery.Schema{
		Name: "test",
		Tables: map[string]*discovery.Table{
			"users": {
				Name:        "users",
				ColumnOrder: []string{"id", "email", "password", "name"},
				Columns: map[string]*discovery.Column{
					"id":       {Name: "id", DataType: "int", IsPrimaryKey: true},
					"email":    {Name: "email", DataType: "varchar(255)"},
					"password": {Name: "password", DataType: "varchar(255)"},
					"name":     {Name: "name", DataType: "varchar(100)"},
				},
				RowCount: 100,
			},
		},
	}

	cfg := &config.Config{
		Settings: config.Settings{
			PreservePrimaryKeys: true,
			Deterministic:       true,
		},
		Tables: map[string]config.TableConfig{
			"users": {
				Columns: map[string]config.ColumnConfig{
					"email": {Strategy: "fake_email"},
				},
			},
		},
	}

	detections := map[string]string{
		"users.email":    "email",
		"users.password": "credential",
		"users.name":     "person_name",
	}

	plan, err := pipeline.CompilePlan(schema, cfg, detections)
	if err != nil {
		t.Fatalf("unexpected plan error: %v", err)
	}

	if plan.TotalRows != 100 {
		t.Errorf("expected 100 rows, got %d", plan.TotalRows)
	}

	tbl := plan.Tables[0]
	colMap := make(map[string]pipeline.ColumnPlan)
	for _, c := range tbl.Columns {
		colMap[c.ColumnName] = c
	}

	// id must be preserved
	if colMap["id"].Strategy != "preserve" {
		t.Errorf("expected id to be preserve, got %s", colMap["id"].Strategy)
	}
	// email specified in config
	if colMap["email"].Strategy != "fake_email" {
		t.Errorf("expected email to be fake_email, got %s", colMap["email"].Strategy)
	}
	// password detected as sensitive without explicit rule must default to redact
	if colMap["password"].Strategy != "redact" {
		t.Errorf("expected password to default to redact, got %s", colMap["password"].Strategy)
	}
}

func TestCompilePlanWithPresetAndSubsetting(t *testing.T) {
	schema := &discovery.Schema{
		Name: "test",
		Tables: map[string]*discovery.Table{
			"users": {
				Name:        "users",
				ColumnOrder: []string{"id", "email", "password", "remember_token"},
				Columns: map[string]*discovery.Column{
					"id":             {Name: "id", DataType: "int", IsPrimaryKey: true},
					"email":          {Name: "email", DataType: "varchar(255)"},
					"password":       {Name: "password", DataType: "varchar(255)"},
					"remember_token": {Name: "remember_token", DataType: "varchar(100)"},
				},
				RowCount: 1000,
			},
		},
	}

	limit := int64(250)
	cfg := &config.Config{
		Settings: config.Settings{
			PreservePrimaryKeys: true,
			Preset:              "laravel",
			SubsetLimit:         &limit,
		},
	}

	plan, err := pipeline.CompilePlan(schema, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected plan compile error: %v", err)
	}

	if plan.TotalRows != 250 {
		t.Errorf("expected 250 subset rows, got %d", plan.TotalRows)
	}

	tbl := plan.Tables[0]
	colMap := make(map[string]pipeline.ColumnPlan)
	for _, c := range tbl.Columns {
		colMap[c.ColumnName] = c
	}

	// In Laravel preset, password -> dev_auth, remember_token -> null
	if colMap["password"].Strategy != "dev_auth" {
		t.Errorf("expected laravel password strategy dev_auth, got %s", colMap["password"].Strategy)
	}
	if colMap["remember_token"].Strategy != "null" {
		t.Errorf("expected laravel remember_token strategy null, got %s", colMap["remember_token"].Strategy)
	}
}
