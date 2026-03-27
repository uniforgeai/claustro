//go:build linux

package clipboard

import (
	"os"
	"os/exec"
	"strings"
)

// NewPlatformHandler returns the Linux clipboard handler (uses xclip or wl-paste from host).
func NewPlatformHandler() PlatformHandler {
	return &linuxHandler{}
}

type linuxHandler struct{}

func (h *linuxHandler) hasDisplay() bool  { return os.Getenv("DISPLAY") != "" }
func (h *linuxHandler) hasWayland() bool  { return os.Getenv("WAYLAND_DISPLAY") != "" }

func (h *linuxHandler) Types() ([]string, error) {
	if h.hasDisplay() {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
		if err == nil {
			if types := imageTypes(string(out)); len(types) > 0 {
				return types, nil
			}
		}
	}
	if h.hasWayland() {
		out, err := exec.Command("wl-paste", "-l").Output()
		if err == nil {
			return imageTypes(string(out)), nil
		}
	}
	return nil, nil
}

func (h *linuxHandler) ReadImage() ([]byte, error) {
	if h.hasDisplay() {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o").Output()
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}
	if h.hasWayland() {
		out, err := exec.Command("wl-paste", "--type", "image/png").Output()
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}
	return nil, nil
}

func (h *linuxHandler) ReadText() (string, error) {
	if h.hasDisplay() {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "text/plain", "-o").Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	if h.hasWayland() {
		out, err := exec.Command("wl-paste").Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", nil
}

// imageTypes filters a MIME-type list (one per line) to image/* entries only.
func imageTypes(output string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "image/") {
			out = append(out, line)
		}
	}
	return out
}
