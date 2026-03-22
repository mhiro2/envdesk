package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/config"
	"github.com/mhiro2/envdesk/internal/crypto"
)

type memberAction func(ctx context.Context, project *config.Project, adapter crypto.Adapter, opts app.MemberOptions) (*app.MemberResult, error)

func newMemberCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Short: "Manage team recipients for encrypted files",
		Example: `  envdesk member add alice.pub --scope api --rekey
  envdesk member remove age1alice... --json`,
	}

	cmd.AddCommand(newMemberAddCommand(newCryptoAdapter), newMemberRemoveCommand(newCryptoAdapter))

	return cmd
}

func newMemberAddCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	return newMemberSubcommand(
		newCryptoAdapter,
		"add",
		"Add a recipient to .sops.yaml",
		`  envdesk member add alice.pub
  envdesk member add age1alice... --scope api --rekey
  envdesk member add alice.pub --json`,
		"add member recipient",
		app.MemberAdd,
	)
}

func newMemberRemoveCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	return newMemberSubcommand(
		newCryptoAdapter,
		"remove",
		"Remove a recipient from .sops.yaml",
		`  envdesk member remove alice.pub
  envdesk member remove age1alice... --scope api --rekey
  envdesk member remove age1alice... --json`,
		"remove member recipient",
		app.MemberRemove,
	)
}

func newMemberSubcommand(newCryptoAdapter cryptoAdapterFactory, use, short, example, errPrefix string, action memberAction) *cobra.Command {
	var scope string
	var rekey bool
	var dryRun bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     use + " <recipient>",
		Short:   short,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := action(cmd.Context(), project, newCryptoAdapter(), app.MemberOptions{
				Recipient: args[0],
				Scope:     scope,
				Rekey:     rekey,
				DryRun:    dryRun,
			})
			if err != nil && result == nil {
				return fmt.Errorf("%s: %w", errPrefix, err)
			}

			if jsonOutput {
				if result == nil {
					return fmt.Errorf("%s: empty result", errPrefix)
				}
				if writeErr := writeJSON(cmd.OutOrStdout(), result); writeErr != nil {
					return writeErr
				}
				if err != nil {
					return fmt.Errorf("%s: %w", errPrefix, err)
				}
				return nil
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", formatProjectPath(project.BaseDir, result.ConfigPath)); err != nil {
				return fmt.Errorf("write member output: %w", err)
			}
			for i, path := range result.AffectedFiles {
				line := fmt.Sprintf("[%d/%d] target %s", i+1, len(result.AffectedFiles), formatProjectPath(project.BaseDir, path))
				if dryRun {
					line += " (dry-run)"
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
					return fmt.Errorf("write member output: %w", err)
				}
			}
			for i, path := range result.RekeyedFiles {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "[%d/%d] rekeyed %s\n", i+1, len(result.RekeyedFiles), formatProjectPath(project.BaseDir, path)); err != nil {
					return fmt.Errorf("write member output: %w", err)
				}
			}
			if err := writeRekeyErrorSummary(cmd.ErrOrStderr(), project.BaseDir, result.Errors); err != nil {
				return err
			}
			if err != nil {
				return fmt.Errorf("%s: %w", errPrefix, err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Limit recipient changes to a specific service")
	cmd.Flags().BoolVar(&rekey, "rekey", false, "Re-encrypt affected files immediately")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview recipient changes without writing files")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable output")

	_ = cmd.RegisterFlagCompletionFunc("scope", completeServiceFlag)

	return cmd
}
