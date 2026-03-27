//go:build darwin

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// NewPlatformHandler returns the macOS clipboard handler (uses osascript).
func NewPlatformHandler() PlatformHandler {
	return &darwinHandler{}
}

type darwinHandler struct{}

func (h *darwinHandler) Types() ([]string, error) {
	cmd := exec.Command("osascript", "-e", "the clipboard as «class PNGf»")
	if err := cmd.Run(); err != nil {
		return nil, nil //nolint:nilerr
	}
	return []string{"image/png", "text/plain"}, nil
}

func (h *darwinHandler) ReadImage() ([]byte, error) {
	f, err := os.CreateTemp("", "claustro-cb-*.png")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	defer os.Remove(name) //nolint:errcheck

	script := fmt.Sprintf(
		"set png_data to (the clipboard as «class PNGf»)\n"+
			"set fp to open for access POSIX file %q with write permission\n"+
			"write png_data to fp\n"+
			"close access fp",
		name,
	)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return nil, fmt.Errorf("writing clipboard image to file: %w", err)
	}
	return os.ReadFile(name)
}

func (h *darwinHandler) ReadText() (string, error) {
	out, err := exec.Command("osascript", "-e", "the clipboard as string").Output()
	if err != nil {
		return "", nil //nolint:nilerr
	}
	return strings.TrimSpace(string(out)), nil
}
