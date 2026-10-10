package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/detector"
	"github.com/HybridMind1337/DataMask/internal/pipeline"
)

func newWizardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wizard",
		Short: "Interactive setup and masking wizard for first-time users",
		Long:  "Guides you interactively through connecting to your database, scanning sensitive fields, and generating safe copies.",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			fmt.Println("==================================================")
			fmt.Println("DataMask Interactive Setup Wizard")
			fmt.Println("==================================================")
			fmt.Println()

			// 1. Database Driver
			fmt.Print("1. Select database engine [1: PostgreSQL, 2: MySQL] (default 1): ")
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)
			driver := "postgres"
			defaultPort := 5432
			if choice == "2" {
				driver = "mysql"
				defaultPort = 3306
			}

			// 2. Source host
			fmt.Print("2. Source Database Host (default localhost): ")
			host, _ := reader.ReadString('\n')
			host = strings.TrimSpace(host)
			if host == "" {
				host = "localhost"
			}

			// 3. Source DB Name
			fmt.Print("3. Source Database Name: ")
			dbName, _ := reader.ReadString('\n')
			dbName = strings.TrimSpace(dbName)
			if dbName == "" {
				return fmt.Errorf("database name cannot be empty")
			}

			// 4. Source User
			fmt.Printf("4. Source Username (default %s): ", driver)
			user, _ := reader.ReadString('\n')
			user = strings.TrimSpace(user)
			if user == "" {
				if driver == "postgres" {
					user = "postgres"
				} else {
					user = "root"
				}
			}

			// 5. Source Password
			fmt.Print("5. Source Password: ")
			pass, _ := reader.ReadString('\n')
			pass = strings.TrimSpace(pass)

			sourceCfg := config.DatabaseConfig{
				Driver:   driver,
				Host:     host,
				Port:     defaultPort,
				Database: dbName,
				Username: user,
				Password: pass,
				SSLMode:  "disable",
				Schema:   "public",
			}

			ctx := context.Background()
			factory := &pipeline.Factory{}

			fmt.Println("\nTesting connection to source database (read-only)...")
			source, err := factory.ConnectSource(ctx, sourceCfg)
			if err != nil {
				return fmt.Errorf("connection failed: %w", err)
			}
			defer source.Close()
			fmt.Println("✓ Connection successful!")

			fmt.Println("\nScanning schema for sensitive fields...")
			schema, err := source.Inspector().InspectSchema(ctx, sourceCfg.Schema)
			if err != nil {
				return fmt.Errorf("schema inspection failed: %w", err)
			}

			det := detector.NewDetector()
			classifications := det.ScanSchema(ctx, schema)

			var sensitiveCols []detector.Classification
			for _, c := range classifications {
				if c.Level != detector.LevelNone {
					sensitiveCols = append(sensitiveCols, c)
				}
			}

			fmt.Printf("✓ Discovered %d tables and %d columns.\n", len(schema.Tables), len(classifications))
			fmt.Printf("✓ Identified %d sensitive fields requiring privacy masking.\n\n", len(sensitiveCols))

			for _, sc := range sensitiveCols {
				fmt.Printf("  • %s.%s -> recommended strategy: %s (%s)\n",
					sc.TableName, sc.ColumnName, sc.RecommendedRule, sc.Level)
			}

			fmt.Println()
			fmt.Print("Generate 'datamask.yaml' now? (Y/n): ")
			genAns, _ := reader.ReadString('\n')
			genAns = strings.TrimSpace(strings.ToLower(genAns))
			if genAns == "" || genAns == "y" || genAns == "yes" {
				tblConfigs := make(map[string]config.TableConfig)
				for _, sc := range classifications {
					tblCfg := tblConfigs[sc.TableName]
					if tblCfg.Columns == nil {
						tblCfg.Columns = make(map[string]config.ColumnConfig)
					}
					if sc.Level != detector.LevelNone {
						detFlag := true
						tblCfg.Columns[sc.ColumnName] = config.ColumnConfig{
							Strategy:      sc.RecommendedRule,
							Deterministic: &detFlag,
						}
					} else {
						tblCfg.Columns[sc.ColumnName] = config.ColumnConfig{
							Strategy: "preserve",
						}
					}
					tblConfigs[sc.TableName] = tblCfg
				}

				targetCfg := sourceCfg
				targetCfg.Database = dbName + "_safe"

				cfg := &config.Config{
					Version: 1,
					Source:  sourceCfg,
					Target:  targetCfg,
					Settings: config.Settings{
						Deterministic:         true,
						DeterministicKeyEnv:   "DATAMASK_TRANSFORM_KEY",
						PreservePrimaryKeys:   true,
						BatchSize:             5000,
						Workers:               4,
						ProtectedEnvironments: []string{"prod", "production", "live"},
					},
					Tables: tblConfigs,
				}

				data, _ := yaml.Marshal(cfg)
				_ = os.WriteFile("datamask.yaml", data, 0644)
				fmt.Println("\n✓ Generated 'datamask.yaml' successfully!")
				fmt.Println("To run your first dry-run, execute:")
				fmt.Println("  datamask mask --dry-run")
			}

			return nil
		},
	}
}
