// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Package clipboard provides an HTTP server that bridges the host clipboard
// into claustro sandbox containers. On Linux it uses a Unix socket; on macOS
// (where Docker containers run in a VM and cannot reach host Unix sockets) it
// uses TCP on 127.0.0.1 with port discovery via a file in the bind-mounted dir.
package clipboard

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// PortFileName is the name of the file written to the clipboard socket directory
// containing the TCP port number. Container-side shims read this file to discover
// how to reach the clipboard server on the host.
const PortFileName = "clipboard.port"

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

// Server is an HTTP server exposing the host clipboard to a container.
type Server struct {
	handler  PlatformHandler
	sockPath string
	portFile string
	listener net.Listener
	srv      *http.Server
}

// New creates a new Server backed by handler.
func New(handler PlatformHandler) *Server {
	return &Server{handler: handler}
}

// Start starts the server, creating a Unix socket at sockPath.
// Use this on Linux where bind-mounted Unix sockets work across namespaces.
func (s *Server) Start(sockPath string) error {
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listening on clipboard socket: %w", err)
	}
	// Make the socket world-readable+writable so the sandbox user (uid 1000)
	// can connect even though the socket is owned by the host user's uid.
	if err := os.Chmod(sockPath, 0o666); err != nil {
		ln.Close() //nolint:errcheck
		return fmt.Errorf("setting clipboard socket permissions: %w", err)
	}
	s.sockPath = sockPath
	s.listener = ln

	s.serve()
	return nil
}

// StartTCP starts the server on TCP 127.0.0.1:0 (OS-assigned port) and writes
// the port number to a file named clipboard.port in dir. Container-side shims
// read this file and connect via host.docker.internal:<port>.
// Use this on macOS where Unix sockets cannot cross the VM boundary.
func (s *Server) StartTCP(dir string) (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listening on clipboard TCP: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s.listener = ln

	portFile := filepath.Join(dir, PortFileName)
	if err := os.WriteFile(portFile, []byte(strconv.Itoa(port)), 0o644); err != nil {
		ln.Close() //nolint:errcheck
		return 0, fmt.Errorf("writing clipboard port file: %w", err)
	}
	s.portFile = portFile

	s.serve()
	return port, nil
}

func (s *Server) serve() {
	mux := http.NewServeMux()
	mux.HandleFunc("/types", s.handleTypes)
	mux.HandleFunc("/image/png", s.handleImagePNG)
	mux.HandleFunc("/text", s.handleText)

	s.srv = &http.Server{Handler: mux}
	go s.srv.Serve(s.listener) //nolint:errcheck
}

// Close shuts down the server and removes the socket/port files.
func (s *Server) Close() error {
	var closeErr error
	if s.srv != nil {
		closeErr = s.srv.Close()
	}
	if s.sockPath != "" {
		os.Remove(s.sockPath) //nolint:errcheck
	}
	if s.portFile != "" {
		os.Remove(s.portFile) //nolint:errcheck
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
