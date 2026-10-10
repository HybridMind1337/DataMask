package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/database/mysql"
	"github.com/HybridMind1337/DataMask/internal/database/postgres"
	"github.com/HybridMind1337/DataMask/internal/database/sqlite"
	"github.com/HybridMind1337/DataMask/internal/discovery"
	"github.com/HybridMind1337/DataMask/internal/graph"
)

// Factory connects to source and target databases.
type Factory struct{}

// ConnectSource establishes a read-only session to the source database.
func (f *Factory) ConnectSource(ctx context.Context, cfg config.DatabaseConfig) (database.Driver, error) {
	switch strings.ToLower(cfg.Driver) {
	case "postgres":
		return postgres.NewDriver(ctx, cfg, true) // read-only enforced
	case "mysql", "mariadb":
		return mysql.NewDriver(ctx, cfg, true)    // read-only enforced
	case "sqlite":
		return sqlite.NewDriver(ctx, cfg, true)   // read-only enforced
	default:
		return nil, fmt.Errorf("unsupported source driver: %s", cfg.Driver)
	}
}

// ConnectTarget establishes a read-write session to the target database.
func (f *Factory) ConnectTarget(ctx context.Context, cfg config.DatabaseConfig) (database.Driver, error) {
	switch strings.ToLower(cfg.Driver) {
	case "postgres":
		return postgres.NewDriver(ctx, cfg, false)
	case "mysql", "mariadb":
		return mysql.NewDriver(ctx, cfg, false)
	case "sqlite":
		return sqlite.NewDriver(ctx, cfg, false)
	default:
		return nil, fmt.Errorf("unsupported target driver: %s", cfg.Driver)
	}
}

// Plan represents the complete compiled masking execution plan.
type Plan struct {
	Tables         []TablePlan `json:"tables" yaml:"tables"`
	TotalRows      int64       `json:"total_rows" yaml:"total_rows"`
	TotalColumns   int         `json:"total_columns" yaml:"total_columns"`
	SensitiveCount int         `json:"sensitive_count" yaml:"sensitive_count"`
	ExecutionOrder []string    `json:"execution_order" yaml:"execution_order"`
	Warnings       []string    `json:"warnings" yaml:"warnings"`
}

// TablePlan holds compiled instructions for a single table.
type TablePlan struct {
	TableName string       `json:"table_name" yaml:"table_name"`
	RowCount  int64        `json:"row_count" yaml:"row_count"`
	Limit     *int64       `json:"limit,omitempty" yaml:"limit,omitempty"`
	Columns   []ColumnPlan `json:"columns" yaml:"columns"`
}

// ColumnPlan holds compiled transformation rule for a single column.
type ColumnPlan struct {
	ColumnName    string `json:"column_name" yaml:"column_name"`
	DataType      string `json:"data_type" yaml:"data_type"`
	Strategy      string `json:"strategy" yaml:"strategy"`
	Deterministic bool   `json:"deterministic" yaml:"deterministic"`
	RiskLevel     string `json:"risk_level" yaml:"risk_level"`
	DetectedType  string `json:"detected_type" yaml:"detected_type"`
	IsFreeText    bool   `json:"is_free_text" yaml:"is_free_text"`
}

// CompilePlan builds an execution plan from schema, detection results, and configuration.
func CompilePlan(schema *discovery.Schema, cfg *config.Config, detections map[string]string) (*Plan, error) {
	depGraph := graph.NewDependencyGraph(schema)
	order, cycles, _ := depGraph.TopologicalSort()
	var warnings []string
	if len(cycles) > 0 {
		warnings = append(warnings, fmt.Sprintf("Cyclic foreign keys detected in %s; constraint checks will be deferred", strings.Join(cycles, ", ")))
		// Append remaining cycles to end of execution order
		order = append(order, cycles...)
	}

	plan := &Plan{
		ExecutionOrder: order,
		Warnings:       warnings,
	}

	// Filter excluded/included tables
	includedSet := make(map[string]bool)
	hasIncludes := len(cfg.Settings.IncludeTables) > 0
	for _, inc := range cfg.Settings.IncludeTables {
		includedSet[inc] = true
	}
	excludedSet := make(map[string]bool)
	for _, exc := range cfg.Settings.ExcludeTables {
		excludedSet[exc] = true
	}

	for _, tableName := range order {
		table, ok := schema.Tables[tableName]
		if !ok {
			continue
		}
		if hasIncludes && !includedSet[tableName] {
			continue
		}
		if excludedSet[tableName] {
			continue
		}

		tblCfg := cfg.Tables[tableName]
		var colPlans []ColumnPlan

		for _, colName := range table.ColumnOrder {
			col := table.Columns[colName]
			plan.TotalColumns++

			// 1. Explicit config rule
			strategy := ""
			det := cfg.Settings.Deterministic
			if colCfg, ok := tblCfg.Columns[colName]; ok {
				strategy = colCfg.Strategy
				if colCfg.Deterministic != nil {
					det = *colCfg.Deterministic
				}
			}

			// 2. Framework preset rules (if preset is configured or matched)
			if strategy == "" && cfg.Settings.Preset != "" {
				if p, ok := config.GetPreset(cfg.Settings.Preset); ok {
					if tRules, ok := p.Rules[tableName]; ok {
						if rule, ok := tRules[colName]; ok {
							strategy = rule
						}
					}
				}
			}

			// 3. Primary key protection
			if strategy == "" && col.IsPrimaryKey && cfg.Settings.PreservePrimaryKeys {
				strategy = "preserve"
			}

			// 3. Fallback to detection recommendation
			detKey := tableName + "." + colName
			detType := detections[detKey]
			risk := "NONE"
			if detType != "" && detType != "operational" {
				risk = "HIGH"
				plan.SensitiveCount++
			}

			if strategy == "" {
				if detType != "" && detType != "operational" {
					strategy = "redact" // safe default for detected sensitive field
				} else {
					strategy = "preserve"
				}
			}

			colPlans = append(colPlans, ColumnPlan{
				ColumnName:    colName,
				DataType:      col.DataType,
				Strategy:      strategy,
				Deterministic: det,
				RiskLevel:     risk,
				DetectedType:  detType,
			})
		}

		// Check subset limits
		var tableLimit *int64
		if tblCfg.Limit != nil {
			tableLimit = tblCfg.Limit
		} else if cfg.Settings.SubsetLimit != nil {
			tableLimit = cfg.Settings.SubsetLimit
		} else if cfg.Settings.SubsetPercent != nil && *cfg.Settings.SubsetPercent > 0 && *cfg.Settings.SubsetPercent < 100 {
			calc := int64(float64(table.RowCount) * (*cfg.Settings.SubsetPercent / 100.0))
			if calc <= 0 && table.RowCount > 0 {
				calc = 1
			}
			tableLimit = &calc
		}

		effectiveRows := table.RowCount
		if tableLimit != nil && *tableLimit < effectiveRows {
			effectiveRows = *tableLimit
		}

		plan.Tables = append(plan.Tables, TablePlan{
			TableName: tableName,
			RowCount:  effectiveRows,
			Limit:     tableLimit,
			Columns:   colPlans,
		})
		plan.TotalRows += effectiveRows
	}

	return plan, nil
}
