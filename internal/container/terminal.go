package container

import (
	"os"

	"golang.org/x/term"
)

var savedState *term.State

func setRawTerminal() error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	savedState = state
	return nil
}

func restoreTerminal() {
	if savedState != nil {
		term.Restore(int(os.Stdin.Fd()), savedState) //nolint:errcheck
		savedState = nil
	}
}
