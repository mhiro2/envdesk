package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const editPathEnvVar = "ENVDESK_EDIT_PATH"

type editorShell struct {
	Command      string
	Args         []string
	PathArgument string
}

func buildEditorCommand(ctx context.Context, editor, path string, shell editorShell) (*exec.Cmd, error) {
	trimmedEditor := strings.TrimSpace(editor)
	if trimmedEditor == "" {
		return nil, fmt.Errorf("run editor: empty editor")
	}

	commandArgs := make([]string, 0, len(shell.Args)+1)
	commandArgs = append(commandArgs, shell.Args...)
	commandArgs = append(commandArgs, trimmedEditor+" "+shell.PathArgument)

	// #nosec G204 -- the editor command is selected explicitly by the user.
	command := exec.CommandContext(ctx, shell.Command, commandArgs...)
	command.Env = append(os.Environ(), editPathEnvVar+"="+path)

	return command, nil
}
