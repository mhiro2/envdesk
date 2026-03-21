//go:build !windows

package app

func currentEditorShell() editorShell {
	return editorShell{
		Command:      "/bin/sh",
		Args:         []string{"-c"},
		PathArgument: `"$` + editPathEnvVar + `"`,
	}
}
