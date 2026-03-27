//go:build windows

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// NewPlatformHandler returns the Windows clipboard handler (uses PowerShell).
func NewPlatformHandler() PlatformHandler {
	return &windowsHandler{}
}

type windowsHandler struct{}

func (h *windowsHandler) Types() ([]string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-Clipboard -Format Image) -ne $null")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil //nolint:nilerr
	}
	if strings.TrimSpace(string(out)) == "True" {
		return []string{"image/png"}, nil
	}
	return nil, nil
}

func (h *windowsHandler) ReadImage() ([]byte, error) {
	f, err := os.CreateTemp("", "claustro-cb-*.png")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	name := f.Name()
	f.Close()
	defer os.Remove(name) //nolint:errcheck

	script := fmt.Sprintf(
		`$img = Get-Clipboard -Format Image; if ($img) { $img.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png) }`,
		strings.ReplaceAll(name, `\`, `\\`),
	)
	if err := exec.Command("powershell", "-NoProfile", "-Command", script).Run(); err != nil {
		return nil, fmt.Errorf("saving clipboard image: %w", err)
	}
	return os.ReadFile(name)
}

func (h *windowsHandler) ReadText() (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
	if err != nil {
		return "", nil //nolint:nilerr
	}
	return strings.TrimSpace(string(out)), nil
}
