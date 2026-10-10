package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/detector"
	"github.com/HybridMind1337/DataMask/internal/masking"
	"github.com/HybridMind1337/DataMask/internal/masking/transformers"
	"github.com/HybridMind1337/DataMask/internal/pipeline"
	"github.com/HybridMind1337/DataMask/internal/reporting"
	"github.com/HybridMind1337/DataMask/internal/ui"
)

func newDumpCmd() *cobra.Command {
	var (
		sourceURL     string
		outputFile    string
		preset        string
		subsetPercent float64
		subsetRows    int64
		reportJSON    bool
	)

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Stream sanitised data directly into an SQL or compressed (.sql.gz) dump file",
		Long: `Extracts data from the source database, runs the masking engine on the fly,
and streams standard SQL INSERT statements directly to disk or Gzip compression.
No secondary target database is required.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputFile == "" {
				outputFile = "datamask_export.sql.gz"
			}

			var cfg *config.Config
			var err error

			if sourceURL != "" {
				sCfg, err := config.ParseDSN(sourceURL)
				if err != nil {
					return fmt.Errorf("parsing source dsn: %w", err)
				}
				cfg = &config.Config{
					Version: 1,
					Source:  sCfg,
				}
				cfg.SetDefaults()
			} else {
				cfg, err = config.Load(cfgFile)
				if err != nil {
					return err
				}
			}

			if preset != "" {
				cfg.Settings.Preset = preset
			}
			if subsetRows > 0 {
				cfg.Settings.SubsetLimit = &subsetRows
			}
			if subsetPercent > 0 {
				cfg.Settings.SubsetPercent = &subsetPercent
			}

			ctx := context.Background()
			factory := &pipeline.Factory{}
			source, err := factory.ConnectSource(ctx, cfg.Source)
			if err != nil {
				return fmt.Errorf("connecting to source: %w", err)
			}
			defer source.Close()

			schema, err := source.Inspector().InspectSchema(ctx, cfg.Source.Schema)
			if err != nil {
				return err
			}

			det := detector.NewDetector()
			classifications := det.ScanSchema(ctx, schema)
			detMap := make(map[string]string)
			for _, c := range classifications {
				detMap[c.TableName+"."+c.ColumnName] = c.DetectedType
			}

			plan, err := pipeline.CompilePlan(schema, cfg, detMap)
			if err != nil {
				return err
			}

			dumpWriter, err := database.NewDumpWriter(outputFile, cfg.Source.Driver)
			if err != nil {
				return err
			}
			defer dumpWriter.Close()

			dumpDriver := database.NewDumpDriver(dumpWriter)

			reg := masking.NewRegistry()
			if err := transformers.RegisterBuiltins(reg); err != nil {
				return err
			}

			orchestrator := pipeline.NewOrchestrator(reg, cfg)

			ui.Banner(os.Stdout, "Direct SQL Dump Streamer")
			ui.Info("Streaming from \033[1m%s\033[0m (%s) -> \033[1m%s\033[0m...", cfg.Source.Database, cfg.Source.Driver, outputFile)
			fmt.Println()

			stats, err := orchestrator.Execute(ctx, source, dumpDriver, schema, plan, func(ev pipeline.ProgressEvent) {
				if !quiet {
					ui.StreamProgress(ev.CurrentTable, ev.TableDone, ev.TableRows, ev.RowsPerSec, 0)
				}
			})
			if err != nil {
				return fmt.Errorf("dump streaming failed: %w", err)
			}

			fmt.Println()
			ui.Success("Dump file generated successfully: %s", outputFile)

			rep := &reporting.AuditReport{
				RunID:           reporting.GenerateRunID(),
				Version:         version,
				StartedAt:       stats.StartTime,
				CompletedAt:     stats.StartTime.Add(stats.Duration),
				DurationSec:     stats.Duration.Seconds(),
				SourceDriver:    cfg.Source.Driver,
				TargetDriver:    "sql_dump_file",
				TablesProcessed: stats.TablesProcessed,
				RowsProcessed:   stats.RowsProcessed,
				FieldsMasked:    stats.FieldsMasked,
				Status:          "SUCCESS",
			}

			if reportJSON {
				return rep.WriteJSON(os.Stdout)
			}

			rep.WriteConsole(os.Stdout)
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceURL, "source", "", "Source database connection URL (e.g. postgres://...)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "datamask_export.sql.gz", "Destination path (.sql or .sql.gz)")
	cmd.Flags().StringVar(&preset, "preset", "", "Framework preset to apply (laravel, django, rails, wordpress)")
	cmd.Flags().Float64Var(&subsetPercent, "subset-percent", 0, "Extract and mask only a percentage of records (e.g. 10 for 10%)")
	cmd.Flags().Int64Var(&subsetRows, "subset-rows", 0, "Extract and mask up to N rows per table (e.g. 5000)")
	cmd.Flags().BoolVar(&reportJSON, "json", false, "Output audit report as JSON")

	return cmd
}
