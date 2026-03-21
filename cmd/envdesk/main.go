package main

import (
	"os"

	"github.com/mhiro2/envdesk/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		cli.PrintErrorHint(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
