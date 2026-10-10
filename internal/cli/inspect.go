package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/pipeline"
)

func newInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [table]",
		Short: "Inspect structure and column statistics of a table",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tableName := args[0]
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

			table, err := source.Inspector().InspectTable(ctx, cfg.Source.Schema, tableName)
			if err != nil {
				return err
			}

			fmt.Printf("Table: %s (Rows: %d)\n", table.Name, table.RowCount)
			fmt.Println("──────────────────────────────────────────")
			fmt.Printf("%-20s %-15s %-10s %-8s\n", "Column", "Type", "Nullable", "Key")
			for _, colName := range table.ColumnOrder {
				col := table.Columns[colName]
				keyStr := ""
				if col.IsPrimaryKey {
					keyStr = "PK"
				} else if col.IsForeignKey {
					keyStr = "FK"
				} else if col.IsUnique {
					keyStr = "UQ"
				}
				nullStr := "YES"
				if !col.IsNullable {
					nullStr = "NO"
				}
				fmt.Printf("%-20s %-15s %-10s %-8s\n", col.Name, col.DataType, nullStr, keyStr)
			}
			return nil
		},
	}
}
