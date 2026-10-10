package pipeline

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
	"github.com/HybridMind1337/DataMask/internal/masking"
	"github.com/HybridMind1337/DataMask/internal/security"
)

// ProgressEvent communicates live streaming metrics.
type ProgressEvent struct {
	CurrentTable string
	TableRows    int64
	TableDone    int64
	TotalDone    int64
	TotalRows    int64
	RowsPerSec   float64
	MemoryMB     uint64
	Finished     bool
	TableErr     error
}

// Stats tracks aggregate execution performance.
type Stats struct {
	TablesProcessed int
	RowsProcessed   int64
	FieldsMasked    int64
	StartTime       time.Time
	Duration        time.Duration
}

// Orchestrator executes data masking pipelines in dependency order.
type Orchestrator struct {
	Registry *masking.Registry
	Config   *config.Config
}

// NewOrchestrator creates a pipeline orchestrator.
func NewOrchestrator(reg *masking.Registry, cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		Registry: reg,
		Config:   cfg,
	}
}

// Execute streams and transforms tables from source to target.
func (o *Orchestrator) Execute(ctx context.Context, source, target database.Driver, schema *discovery.Schema, plan *Plan, progressCb func(ProgressEvent)) (*Stats, error) {
	// Guard against writing to protected production targets
	guard := security.NewTargetProtectionGuard(o.Config.Settings.ProtectedEnvironments)
	if err := guard.CheckTarget(o.Config.Target.Database, false); err != nil {
		return nil, err
	}

	// Prepare KeyDeriver for deterministic transformations
	var deriver *security.KeyDeriver
	if o.Config.Settings.Deterministic {
		d, err := security.NewKeyDeriver(o.Config.Settings.DeterministicKeyEnv)
		if err != nil {
			// If not found in env, warn or fail according to configuration
			if o.Config.Settings.StopOnError {
				return nil, err
			}
		}
		deriver = d
	}

	tCtx := masking.NewTransformContext(
		o.Config.Settings.Deterministic,
		deriver,
		o.Config.Settings.DevelopmentAuth.FixedHash,
	)

	// Disable foreign keys temporarily on target during bulk load
	_ = target.Constraints().DisableAllForeignKeys(ctx, schema.Name)
	defer func() {
		_ = target.Constraints().EnableAllForeignKeys(ctx, schema.Name)
	}()

	stats := &Stats{
		StartTime: time.Now(),
	}

	var totalProcessed int64
	batchSize := o.Config.Settings.BatchSize
	if batchSize <= 0 {
		batchSize = 5000
	}

	for _, tblPlan := range plan.Tables {
		table, ok := schema.Tables[tblPlan.TableName]
		if !ok {
			continue
		}

		// Ensure target table exists and is clean
		_ = target.Writer().CreateTableFromSource(ctx, table)
		if err := target.Writer().TruncateTable(ctx, table); err != nil {
			// Non-fatal if table was just created
		}

		// Build column transformation map
		strategyMap := make(map[string]masking.Transformer)
		for _, cp := range tblPlan.Columns {
			if cp.Strategy != "" && cp.Strategy != "preserve" {
				if t, ok := o.Registry.Get(cp.Strategy); ok {
					strategyMap[cp.ColumnName] = t
				}
			}
		}

		// Pre-bind compiled column transformers for high-speed row transformation
		type compiledTransform struct {
			colName string
			trans   masking.Transformer
			meta    masking.FieldMetadata
		}
		var boundTransforms []compiledTransform
		for colName, trans := range strategyMap {
			col := table.Columns[colName]
			boundTransforms = append(boundTransforms, compiledTransform{
				colName: colName,
				trans:   trans,
				meta: masking.FieldMetadata{
					SchemaName:   table.Schema,
					TableName:    table.Name,
					ColumnName:   colName,
					DataType:     col.DataType,
					IsNullable:   col.IsNullable,
					IsPrimaryKey: col.IsPrimaryKey,
					IsForeignKey: col.IsForeignKey,
					IsUnique:     col.IsUnique,
				},
			})
		}

		pkCol := ""
		if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0 {
			pkCol = table.PrimaryKey.Columns[0]
		}

		// Stream read from source
		batchChan, errChan := source.Reader().ReadTableStream(ctx, table, batchSize)
		var tableProcessed int64
		tableStart := time.Now()

		for batch := range batchChan {
			// Check if batch exceeds remaining tableLimit
			if tblPlan.Limit != nil {
				remaining := *tblPlan.Limit - tableProcessed
				if remaining <= 0 {
					break
				}
				if int64(len(batch.Rows)) > remaining {
					batch.Rows = batch.Rows[:remaining]
				}
			}

			// Parallel or chunked row transformation with zero metadata reallocation
			var maskedCount int64
			for rIdx := range batch.Rows {
				row := &batch.Rows[rIdx]
				var rowId any
				if pkCol != "" {
					rowId = row.Values[pkCol]
				}

				for i := range boundTransforms {
					bt := &boundTransforms[i]
					meta := bt.meta
					meta.RowIdentifier = rowId

					origVal := row.Values[bt.colName]
					newVal, err := bt.trans.Transform(ctx, origVal, meta, tCtx)
					if err != nil {
						safeErr := security.FormatSafeError(table.Name, bt.colName, rowId, err)
						if o.Config.Settings.StopOnError {
							return nil, safeErr
						}
					} else {
						row.Values[bt.colName] = newVal
						maskedCount++
					}
				}
			}
			atomic.AddInt64(&stats.FieldsMasked, maskedCount)

			// Write transformed batch into target
			if err := target.Writer().WriteBatch(ctx, table, batch); err != nil {
				return nil, fmt.Errorf("writing batch to target for table %s: %w", table.Name, err)
			}

			count := int64(len(batch.Rows))
			tableProcessed += count
			totalProcessed += count

			if progressCb != nil {
				elapsed := time.Since(tableStart).Seconds()
				rate := 0.0
				if elapsed > 0 {
					rate = float64(tableProcessed) / elapsed
				}
				progressCb(ProgressEvent{
					CurrentTable: table.Name,
					TableRows:    tblPlan.RowCount,
					TableDone:    tableProcessed,
					TotalDone:    totalProcessed,
					TotalRows:    plan.TotalRows,
					RowsPerSec:   rate,
				})
			}
		}

		if err := <-errChan; err != nil {
			return nil, fmt.Errorf("reading stream from table %s: %w", table.Name, err)
		}

		stats.TablesProcessed++
	}

	stats.RowsProcessed = totalProcessed
	stats.Duration = time.Since(stats.StartTime)

	return stats, nil
}
