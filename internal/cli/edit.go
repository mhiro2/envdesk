package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/crypto"
)

func newEditCommand() *cobra.Command {
	var editor string
	var noLint bool

	cmd := &cobra.Command{
		Use:   "edit <service> <environment>",
		Short: "Edit an encrypted env file safely",
		Example: `  envdesk edit api dev
  envdesk edit api prod --editor "nvim -u NONE"
  envdesk edit web stg --no-lint`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeServiceEnvArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			result, err := app.Edit(cmd.Context(), project, crypto.NewSOPS(), app.EditOptions{
				Service:     args[0],
				Environment: args[1],
				Editor:      editor,
				SkipLint:    noLint,
			})
			if err != nil {
				return fmt.Errorf("edit env file: %w", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", formatProjectPath(project.BaseDir, result.Path)); err != nil {
				return fmt.Errorf("write edit output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&editor, "editor", "", "Override the editor command")
	cmd.Flags().BoolVar(&noLint, "no-lint", false, "Skip schema validation after editing")

	return cmd
}
