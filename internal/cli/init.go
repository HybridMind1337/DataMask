package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/detector"
	"github.com/HybridMind1337/DataMask/internal/pipeline"
)

func newInitCmd() *cobra.Command {
	var (
		sourceURL  string
		targetURL  string
		outputPath string
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Automatically scan source database and generate a production datamask.yaml",
		Long: `Connects to the source database, runs the multi-signal detection engine on all tables,
and writes a ready-to-use datamask.yaml with safe masking strategies pre-configured.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				outputPath = "datamask.yaml"
			}

			if _, err := os.Stat(outputPath); err == nil && !force {
				return fmt.Errorf("%s already exists. Use --force to overwrite", outputPath)
			}

			var sourceCfg config.DatabaseConfig
			var err error
			if sourceURL != "" {
				sourceCfg, err = config.ParseDSN(sourceURL)
				if err != nil {
					return fmt.Errorf("parsing source url: %w", err)
				}
			} else {
				// Fallback to reading existing or default postgres
				sourceCfg = config.DatabaseConfig{
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					Database: "production_db",
					Username: "postgres",
					Password: "${DB_PASSWORD}",
					Schema:   "public",
				}
			}

			var targetCfg config.DatabaseConfig
			if targetURL != "" {
				targetCfg, err = config.ParseDSN(targetURL)
				if err != nil {
					return fmt.Errorf("parsing target url: %w", err)
				}
			} else {
				targetCfg = config.DatabaseConfig{
					Driver:   sourceCfg.Driver,
					Host:     "localhost",
					Port:     sourceCfg.Port,
					Database: "staging_safe",
					Username: sourceCfg.Username,
					Password: "${TARGET_DB_PASSWORD:-secret}",
					Schema:   sourceCfg.Schema,
				}
			}

			ctx := context.Background()
			factory := &pipeline.Factory{}
			source, err := factory.ConnectSource(ctx, sourceCfg)
			if err != nil {
				return fmt.Errorf("connecting to source: %w", err)
			}
			defer source.Close()

			fmt.Println("DataMask Auto-Discovery")
			fmt.Println("──────────────────────────────────────────")
			fmt.Printf("Introspecting schema from %s (%s)...\n", sourceCfg.Database, sourceCfg.Driver)

			schema, err := source.Inspector().InspectSchema(ctx, sourceCfg.Schema)
			if err != nil {
				return fmt.Errorf("inspecting schema: %w", err)
			}

			det := detector.NewDetector()
			classifications := det.ScanSchema(ctx, schema)

			// Map table and column configs
			tableConfigs := make(map[string]config.TableConfig)
			detCount := 0

			for _, cls := range classifications {
				tblCfg, ok := tableConfigs[cls.TableName]
				if !ok {
					tblCfg = config.TableConfig{
						Columns: make(map[string]config.ColumnConfig),
					}
				}

				if cls.Level != detector.LevelNone {
					detCount++
					detFlag := true
					tblCfg.Columns[cls.ColumnName] = config.ColumnConfig{
						Strategy:      cls.RecommendedRule,
						Deterministic: &detFlag,
					}
				} else {
					tblCfg.Columns[cls.ColumnName] = config.ColumnConfig{
						Strategy: "preserve",
					}
				}

				tableConfigs[cls.TableName] = tblCfg
			}

			genCfg := &config.Config{
				Version: 1,
				Source:  sourceCfg,
				Target:  targetCfg,
				Settings: config.Settings{
					Deterministic:         true,
					DeterministicKeyEnv:   "DATAMASK_TRANSFORM_KEY",
					PreservePrimaryKeys:   true,
					StopOnError:           true,
					BatchSize:             5000,
					Workers:               4,
					ProtectedEnvironments: []string{"prod", "production", "live", "main", "primary"},
					DevelopmentAuth: config.DevAuthConfig{
						Enabled:     true,
						PasswordEnv: "DATAMASK_DEV_PASSWORD",
						FixedHash:   "$2y$10$TKh8H1.PfQx37YgCzwiKb.KjNyWgaHb9cbcoQgdIVFlYg7B77UdFm",
					},
				},
				Tables: tableConfigs,
			}

			outBytes, err := yaml.Marshal(genCfg)
			if err != nil {
				return fmt.Errorf("generating yaml: %w", err)
			}

			if err := os.WriteFile(outputPath, outBytes, 0644); err != nil {
				return fmt.Errorf("writing configuration: %w", err)
			}

			fmt.Println()
			fmt.Printf("Discovered %d tables and %d columns.\n", len(schema.Tables), len(classifications))
			fmt.Printf("Auto-configured %d sensitive fields with privacy rules.\n", detCount)
			fmt.Printf("Configuration saved to: %s\n\n", outputPath)
			fmt.Println("You can now review or run directly:")
			fmt.Printf("  datamask mask --config %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceURL, "source", "", "Source database connection URL (e.g. postgres://user:pass@host:5432/db)")
	cmd.Flags().StringVar(&targetURL, "target", "", "Target database connection URL (e.g. postgres://user:pass@host:5432/dev_db)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "datamask.yaml", "Destination path for generated config file")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration file")

	return cmd
}
