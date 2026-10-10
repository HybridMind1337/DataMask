package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts for datamask",
		Long: `To load completions:

Bash:
  $ source <(datamask completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ datamask completion bash > /etc/bash_completion.d/datamask
  # macOS:
  $ datamask completion bash > $(brew --prefix)/etc/bash_completion.d/datamask

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ datamask completion zsh > "${fpath[1]}/_datamask"

Fish:
  $ datamask completion fish | source

  # To load completions for each session, execute once:
  $ datamask completion fish > ~/.config/fish/completions/datamask.fish

PowerShell:
  PS> datamask completion powershell | Out-String | Invoke-Expression
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell type: %s", args[0])
			}
		},
	}
}
