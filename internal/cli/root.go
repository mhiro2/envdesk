package cli

import (
	"github.com/spf13/cobra"

	"github.com/mhiro2/envdesk/internal/crypto"
)

type cryptoAdapterFactory func() crypto.Adapter

func NewRootCommand() *cobra.Command {
	return newRootCommand(defaultCryptoAdapterFactory)
}

func newRootCommand(adapterFactory cryptoAdapterFactory) *cobra.Command {
	if adapterFactory == nil {
		adapterFactory = defaultCryptoAdapterFactory
	}

	initCmd := newInitCommand(adapterFactory)
	initCmd.GroupID = "setup"

	exampleCmd := newExampleCommand(adapterFactory)
	exampleCmd.GroupID = "daily"

	editCmd := newEditCommand()
	editCmd.GroupID = "daily"

	exportCmd := newExportCommand(adapterFactory)
	exportCmd.GroupID = "daily"

	diffCmd := newDiffCommand(adapterFactory)
	diffCmd.GroupID = "review"

	lintCmd := newLintCommand(adapterFactory)
	lintCmd.GroupID = "review"

	checkSyncCmd := newCheckSyncCommand(adapterFactory)
	checkSyncCmd.GroupID = "review"

	statusCmd := newStatusCommand(adapterFactory)
	statusCmd.GroupID = "review"

	auditCmd := newAuditCommand()
	auditCmd.GroupID = "review"

	doctorCmd := newDoctorCommand()
	doctorCmd.GroupID = "setup"

	syncKeysCmd := newSyncKeysCommand(adapterFactory)
	syncKeysCmd.GroupID = "review"

	memberCmd := newMemberCommand(adapterFactory)
	memberCmd.GroupID = "access"

	rekeyCmd := newRekeyCommand(adapterFactory)
	rekeyCmd.GroupID = "access"

	completionCmd := newCompletionCommand()
	completionCmd.GroupID = "setup"

	cmd := &cobra.Command{
		Use:   "envdesk",
		Short: "Manage env files for teams",
		Long: `envdesk manages encrypted .env files for teams.

It provides a Git-friendly workflow for validating, diffing,
and aligning environment files across services and environments.`,
		Example: `  envdesk init --services api,web --envs dev,stg,prod --sops
  envdesk edit api dev
  envdesk diff api dev stg --show-metadata
  envdesk status --service api
  envdesk check-sync --json`,
		Version:       buildVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd:   true,
			DisableNoDescFlag:   true,
			DisableDescriptions: false,
			HiddenDefaultCmd:    true,
		},
	}

	cmd.AddGroup(
		&cobra.Group{ID: "setup", Title: "Setup Commands:"},
		&cobra.Group{ID: "daily", Title: "Daily Commands:"},
		&cobra.Group{ID: "review", Title: "Review Commands:"},
		&cobra.Group{ID: "access", Title: "Access Commands:"},
	)

	cmd.PersistentFlags().String("config", "envdesk.yaml", "Path to envdesk config file")

	cmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Show detailed output")

	cmd.AddCommand(
		initCmd,
		exampleCmd,
		editCmd,
		exportCmd,
		diffCmd,
		lintCmd,
		checkSyncCmd,
		statusCmd,
		auditCmd,
		doctorCmd,
		syncKeysCmd,
		memberCmd,
		rekeyCmd,
		completionCmd,
	)

	cmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	return cmd
}

func defaultCryptoAdapterFactory() crypto.Adapter {
	return crypto.NewSOPS()
}
