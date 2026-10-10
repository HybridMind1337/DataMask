package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	verbose   bool
	quiet     bool
	version   = "0.1.0"
	commit    = "HEAD"
	buildDate = "2026-10-10"
)

// NewRootCmd builds the root Cobra command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datamask",
		Short: "DataMask — Create safe development and staging databases from production data.",
		Long: `DataMask creates privacy-safe copies of production-like databases for development,
testing and staging while preserving database structure, realistic data characteristics
and relational integrity.

No changes are ever written to the source production database.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "datamask.yaml", "Path to configuration file")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newScanCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newMaskCmd())
	cmd.AddCommand(newDumpCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newRulesCmd())
	cmd.AddCommand(newInspectCmd())
	cmd.AddCommand(newWizardCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

// Execute runs the CLI application.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
