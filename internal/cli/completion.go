package cli

import (
	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for envdesk.

To load completions:

Bash:
  $ source <(envdesk completion bash)
  # Or add to ~/.bashrc:
  $ envdesk completion bash > /etc/bash_completion.d/envdesk

Zsh:
  $ envdesk completion zsh > "${fpath[1]}/_envdesk"
  # Then restart your shell

Fish:
  $ envdesk completion fish | source
  # Or persist:
  $ envdesk completion fish > ~/.config/fish/completions/envdesk.fish

PowerShell:
  PS> envdesk completion powershell | Out-String | Invoke-Expression
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			}
			return nil
		},
	}

	return cmd
}
