package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/app"
)

func newExportCommand(newCryptoAdapter cryptoAdapterFactory) *cobra.Command {
	var out string
	var stdout bool
	var force bool

	cmd := &cobra.Command{
		Use:   "export <service> <environment>",
		Short: "Export a decrypted env file for local use",
		Example: `  envdesk export api dev --out .env.local
  envdesk export api stg --stdout
  envdesk export web prod --out tmp/web.prod.env --force`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeServiceEnvArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdout && out != "" {
				return fmt.Errorf("select export output: use either --out or --stdout")
			}
			if !stdout && out == "" {
				return fmt.Errorf("select export output: use --out or --stdout")
			}

			project, err := loadProject(cmd)
			if err != nil {
				return err
			}

			plaintext, err := app.Export(cmd.Context(), project, newCryptoAdapter(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("export env file: %w", err)
			}

			if stdout {
				if !isQuiet(cmd) {
					fprintWarning(cmd.ErrOrStderr(), "secret values will be written to stdout")
				}
				if _, err := cmd.OutOrStdout().Write(plaintext); err != nil {
					return fmt.Errorf("write export output: %w", err)
				}

				return nil
			}

			if err := ensureExportTarget(out, force); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", out, err)
			}
			if err := os.WriteFile(out, plaintext, 0o600); err != nil {
				return fmt.Errorf("write export file %q: %w", out, err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&out, "out", "", "Output path for decrypted env file")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "Write plaintext env to stdout instead of a file")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite output file if it already exists")

	return cmd
}

func ensureExportTarget(path string, force bool) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("check export target %q: directory already exists", path)
		}
		if !force {
			return fmt.Errorf("check export target %q: file already exists", path)
		}

		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("check export target %q: %w", path, err)
	}

	return nil
}
