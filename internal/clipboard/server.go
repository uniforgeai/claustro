// Package clipboard provides a Unix socket server that bridges the host clipboard
// into claustro sandbox containers via a simple HTTP-over-Unix-socket API.
package clipboard

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

// PlatformHandler reads clipboard data from the host platform.
type PlatformHandler interface {
	// Types returns the image MIME types currently on the clipboard.
	// Returns an empty slice if no image is present.
	Types() ([]string, error)
	// ReadImage returns raw PNG bytes from the clipboard, or nil if unavailable.
	ReadImage() ([]byte, error)
	// ReadText returns plain text from the clipboard, or empty string if unavailable.
	ReadText() (string, error)
}

// Server is a Unix socket HTTP server exposing the host clipboard to a container.
type Server struct {
	handler  PlatformHandler
	sockPath string
	listener net.Listener
	srv      *http.Server
}

// New creates a new Server backed by handler.
func New(handler PlatformHandler) *Server {
	return &Server{handler: handler}
}

// Start starts the server, creating a Unix socket at sockPath.
func (s *Server) Start(sockPath string) error {
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listening on clipboard socket: %w", err)
	}
	s.sockPath = sockPath
	s.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/types", s.handleTypes)
	mux.HandleFunc("/image/png", s.handleImagePNG)
	mux.HandleFunc("/text", s.handleText)

	s.srv = &http.Server{Handler: mux}
	go s.srv.Serve(ln) //nolint:errcheck
	return nil
}

// Close shuts down the server and removes the socket file.
func (s *Server) Close() error {
	var closeErr error
	if s.srv != nil {
		closeErr = s.srv.Close()
	}
	if s.sockPath != "" {
		os.Remove(s.sockPath) //nolint:errcheck
	}
	return closeErr
}

func (s *Server) handleTypes(w http.ResponseWriter, _ *http.Request) {
	types, err := s.handler.Types()
	if err != nil || len(types) == 0 {
		http.Error(w, "no image on clipboard", http.StatusNotFound)
		return
	}
	for _, t := range types {
		fmt.Fprintln(w, t) //nolint:errcheck
	}
}

func (s *Server) handleImagePNG(w http.ResponseWriter, _ *http.Request) {
	data, err := s.handler.ReadImage()
	if err != nil || len(data) == 0 {
		http.Error(w, "no image on clipboard", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(data)
}

func (s *Server) handleText(w http.ResponseWriter, _ *http.Request) {
	text, err := s.handler.ReadText()
	if err != nil || text == "" {
		http.Error(w, "no text on clipboard", http.StatusNotFound)
		return
	}
	fmt.Fprint(w, text) //nolint:errcheck
}
