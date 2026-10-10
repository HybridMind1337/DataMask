package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/detector"
	"github.com/HybridMind1337/DataMask/internal/masking"
	"github.com/HybridMind1337/DataMask/internal/masking/transformers"
	"github.com/HybridMind1337/DataMask/internal/pipeline"
	"github.com/HybridMind1337/DataMask/internal/reporting"
	"github.com/HybridMind1337/DataMask/internal/ui"
	"github.com/HybridMind1337/DataMask/internal/validation"
)

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan source database schema and detect potential sensitive fields",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			ctx := context.Background()
			factory := &pipeline.Factory{}
			source, err := factory.ConnectSource(ctx, cfg.Source)
			if err != nil {
				return fmt.Errorf("connecting to source: %w", err)
			}
			defer source.Close()

			ui.Banner(os.Stdout, "Database PII & Schema Discovery")
			ui.Info("Scanning schema \033[1m%s\033[0m (%s)...", cfg.Source.Schema, cfg.Source.Driver)

			schema, err := source.Inspector().InspectSchema(ctx, cfg.Source.Schema)
			if err != nil {
				return fmt.Errorf("inspecting schema: %w", err)
			}

			det := detector.NewDetector()
			classifications := det.ScanSchema(ctx, schema)

			var totalRows int64
			for _, t := range schema.Tables {
				totalRows += t.RowCount
			}

			fmt.Println()
			ui.KeyValue("Tables", len(schema.Tables))
			ui.KeyValue("Columns", len(classifications))
			ui.KeyValue("Total Rows", totalRows)

			ui.SectionHeader("Identified Sensitive Fields")
			fmt.Println()

			sensitiveCount := 0
			for _, c := range classifications {
				if c.Level != detector.LevelNone {
					sensitiveCount++
					fmt.Printf("  %-20s \033[1m%-24s\033[0m \033[2mstrategy:\033[0m \033[36m%-16s\033[0m \033[90m(%.0f%% confidence)\033[0m\n",
						ui.Badge(string(c.Level)),
						c.TableName+"."+c.ColumnName,
						c.RecommendedRule,
						c.Confidence*100,
					)
				}
			}

			if sensitiveCount == 0 {
				ui.Success("No sensitive fields identified.")
			}

			fmt.Println()
			ui.Success("Scan completed successfully.")
			fmt.Println("\033[2mSource database remained strictly read-only.\033[0m")
			return nil
		},
	}
}

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Generate and preview masking execution plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
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

			ui.Banner(os.Stdout, "Execution Plan Preview")

			for _, t := range plan.Tables {
				fmt.Printf("\n%s%s%s %s(%d rows)%s\n", ui.Bold, t.TableName, ui.Reset, ui.Dim, t.RowCount, ui.Reset)
				for _, c := range t.Columns {
					detStr := ""
					if c.Deterministic {
						detStr = " \033[35m[det]\033[0m"
					}
					stratColor := "\033[36m"
					if c.Strategy == "preserve" {
						stratColor = "\033[90m"
					} else if c.Strategy == "redact" || c.Strategy == "null" {
						stratColor = "\033[33m"
					}
					fmt.Printf("  \033[90m├─\033[0m %-20s %s%-18s%s%s\n",
						c.ColumnName,
						stratColor, c.Strategy, ui.Reset,
						detStr,
					)
				}
			}

			fmt.Println()
			ui.KeyValue("Estimated Rows", plan.TotalRows)
			ui.KeyValue("Columns to Mask", plan.SensitiveCount)
			if len(plan.Warnings) > 0 {
				fmt.Println()
				for _, w := range plan.Warnings {
					ui.Warning("%s", w)
				}
			}
			fmt.Println()
			ui.Info("Dry-run preview complete. No data has been modified.")
			return nil
		},
	}
}

func newMaskCmd() *cobra.Command {
	var (
		dryRun         bool
		forceDangerous bool
		reportJSON     bool
		autoMode       bool
		sourceURL      string
		targetURL      string
		preset         string
		subsetPercent  float64
		subsetRows     int64
	)

	cmd := &cobra.Command{
		Use:   "mask",
		Short: "Execute data masking from source database into target database",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg *config.Config
			var err error

			if sourceURL != "" && targetURL != "" {
				sCfg, err := config.ParseDSN(sourceURL)
				if err != nil {
					return fmt.Errorf("parsing source dsn: %w", err)
				}
				tCfg, err := config.ParseDSN(targetURL)
				if err != nil {
					return fmt.Errorf("parsing target dsn: %w", err)
				}
				cfg = &config.Config{
					Version: 1,
					Source:  sCfg,
					Target:  tCfg,
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

			// Apply CLI subset overrides if provided
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

			// If --auto flag is supplied, ensure detected sensitive fields are automatically mapped
			if autoMode && len(cfg.Tables) == 0 {
				cfg.Tables = make(map[string]config.TableConfig)
				for _, c := range classifications {
					tblCfg := cfg.Tables[c.TableName]
					if tblCfg.Columns == nil {
						tblCfg.Columns = make(map[string]config.ColumnConfig)
					}
					if c.Level != detector.LevelNone {
						detFlag := true
						tblCfg.Columns[c.ColumnName] = config.ColumnConfig{
							Strategy:      c.RecommendedRule,
							Deterministic: &detFlag,
						}
					}
					cfg.Tables[c.TableName] = tblCfg
				}
			}

			plan, err := pipeline.CompilePlan(schema, cfg, detMap)
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Println("DRY RUN: Masking Preview")
				fmt.Println("──────────────────────────────────────────")
				fmt.Printf("Source:               %s (%s)\n", cfg.Source.Database, cfg.Source.Driver)
				fmt.Printf("Target:               %s (%s)\n", cfg.Target.Database, cfg.Target.Driver)
				fmt.Printf("Tables scheduled:     %d\n", len(plan.Tables))
				fmt.Printf("Estimated rows:       %d\n", plan.TotalRows)
				fmt.Printf("Columns to transform: %d\n", plan.SensitiveCount)
				fmt.Println("\nExecution Plan:")
				for _, t := range plan.Tables {
					fmt.Printf("  • %-20s (%d rows)\n", t.TableName, t.RowCount)
				}
				fmt.Println("\nDry-run completed successfully. No data was transformed or written.")
				return nil
			}

			target, err := factory.ConnectTarget(ctx, cfg.Target)
			if err != nil {
				return fmt.Errorf("connecting to target: %w", err)
			}
			defer target.Close()

			reg := masking.NewRegistry()
			if err := transformers.RegisterBuiltins(reg); err != nil {
				return err
			}

			orchestrator := pipeline.NewOrchestrator(reg, cfg)

			ui.Banner(os.Stdout, "Active Masking Stream")
			ui.Info("Transforming \033[1m%s\033[0m -> \033[1m%s\033[0m...", cfg.Source.Database, cfg.Target.Database)
			fmt.Println()

			stats, err := orchestrator.Execute(ctx, source, target, schema, plan, func(ev pipeline.ProgressEvent) {
				if !quiet {
					ui.StreamProgress(ev.CurrentTable, ev.TableDone, ev.TableRows, ev.RowsPerSec, 0)
				}
			})
			if err != nil {
				return fmt.Errorf("masking failed: %w", err)
			}

			fmt.Println()
			ui.Success("Masking pipeline completed successfully.")

			// Post-execution verification
			val := validation.NewValidator(target, schema)
			vRep, _ := val.Validate(ctx)

			rep := &reporting.AuditReport{
				RunID:           reporting.GenerateRunID(),
				Version:         version,
				StartedAt:       stats.StartTime,
				CompletedAt:     stats.StartTime.Add(stats.Duration),
				DurationSec:     stats.Duration.Seconds(),
				SourceDriver:    cfg.Source.Driver,
				TargetDriver:    cfg.Target.Driver,
				TablesProcessed: stats.TablesProcessed,
				RowsProcessed:   stats.RowsProcessed,
				FieldsMasked:    stats.FieldsMasked,
				Validation:      vRep,
				Status:          "SUCCESS",
			}

			if reportJSON {
				return rep.WriteJSON(os.Stdout)
			}

			rep.WriteConsole(os.Stdout)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate masking without writing changes")
	cmd.Flags().BoolVar(&forceDangerous, "force-dangerous-target-override", false, "Allow writing to targets with production names")
	cmd.Flags().BoolVar(&reportJSON, "json", false, "Output audit report as JSON")
	cmd.Flags().BoolVarP(&autoMode, "auto", "a", false, "Automatically infer and apply masking rules for all sensitive columns")
	cmd.Flags().StringVar(&sourceURL, "source", "", "Direct source database connection URL (e.g. postgres://...)")
	cmd.Flags().StringVar(&targetURL, "target", "", "Direct target database connection URL (e.g. postgres://...)")
	cmd.Flags().StringVar(&preset, "preset", "", "Framework preset to apply (laravel, django, rails, wordpress)")
	cmd.Flags().Float64Var(&subsetPercent, "subset-percent", 0, "Extract and mask only a percentage of records (e.g. 10 for 10%)")
	cmd.Flags().Int64Var(&subsetRows, "subset-rows", 0, "Extract and mask up to N rows per table (e.g. 5000)")

	return cmd
}

func newVerifyCmd() *cobra.Command {
	var (
		failOnResiduals bool
		failOnWarnings  bool
		ciMode          bool
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Validate target database for residual PII and relational integrity (supports CI/CD fail-safe)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			ctx := context.Background()
			factory := &pipeline.Factory{}
			target, err := factory.ConnectTarget(ctx, cfg.Target)
			if err != nil {
				return fmt.Errorf("connecting to target: %w", err)
			}
			defer target.Close()

			schema, err := target.Inspector().InspectSchema(ctx, cfg.Target.Schema)
			if err != nil {
				return err
			}

			fmt.Println("Privacy Verification")
			fmt.Println("────────────────────────────────────────")

			val := validation.NewValidator(target, schema)
			rep, err := val.Validate(ctx)
			if err != nil {
				return err
			}

			fmt.Printf("Email addresses:          %d\n", rep.ResidualEmails)
			fmt.Printf("Access tokens:            %d\n", rep.ResidualTokens)
			fmt.Printf("JWT strings:              %d\n", rep.ResidualJWTs)
			fmt.Println()
			if len(rep.FreeTextWarnings) > 0 {
				fmt.Println("Free-text review recommended:")
				for _, w := range rep.FreeTextWarnings {
					fmt.Printf("  ⚠ %s\n", w)
				}
				fmt.Println()
			}
			fmt.Printf("Result:                   %s\n", rep.Status)

			// CI / CD Fail-Safe Threshold Enforcement
			if failOnResiduals || ciMode {
				if rep.ResidualEmails > 0 || rep.ResidualTokens > 0 || rep.ResidualJWTs > 0 {
					return fmt.Errorf("CI FAIL-SAFE: detected %d residual PII items on target database",
						rep.ResidualEmails+rep.ResidualTokens+rep.ResidualJWTs)
				}
			}

			if failOnWarnings {
				if len(rep.FreeTextWarnings) > 0 {
					return fmt.Errorf("CI FAIL-SAFE: detected %d unresolved free-text warnings", len(rep.FreeTextWarnings))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&failOnResiduals, "fail-on-residuals", false, "Exit with error code if any residual PII is detected")
	cmd.Flags().BoolVar(&failOnWarnings, "fail-on-warnings", false, "Exit with error code if free-text warnings exist")
	cmd.Flags().BoolVar(&ciMode, "ci", false, "Strict CI/CD mode (fails immediately on any residual PII or broken constraints)")

	return cmd
}

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage and inspect masking rules and transformers",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all registered transformers",
		Run: func(cmd *cobra.Command, args []string) {
			reg := masking.NewRegistry()
			_ = transformers.RegisterBuiltins(reg)

			fmt.Println("Registered Masking Transformers:")
			fmt.Println("──────────────────────────────────────────")
			for _, name := range reg.List() {
				t, _ := reg.Get(name)
				fmt.Printf("%-24s %s\n", name, t.Description())
			}
		},
	})

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print DataMask version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("DataMask v%s (commit: %s, built: %s)\n", version, commit, buildDate)
		},
	}
}
