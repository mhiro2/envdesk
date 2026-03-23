package main

import (
	"fmt"
	"os"

	"github.com/mhiro2/envdesk/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		cli.PrintErrorHint(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
