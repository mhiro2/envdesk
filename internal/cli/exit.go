package cli

import "errors"

const (
	ExitCodeOK          = 0
	ExitCodeRuntime     = 1
	ExitCodeUsage       = 2
	ExitCodeCheckFailed = 3
)

type exitCoder interface {
	ExitCode() int
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	return e.err
}

func (e *exitError) ExitCode() int {
	return e.code
}

func withExitCode(err error) error {
	if err == nil {
		return nil
	}

	var coded exitCoder
	if errors.As(err, &coded) {
		return err
	}

	return &exitError{
		code: ExitCodeCheckFailed,
		err:  err,
	}
}

func ExitCode(err error) int {
	if err == nil {
		return ExitCodeOK
	}

	var coded exitCoder
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}

	return ExitCodeRuntime
}
