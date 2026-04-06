// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Server is the host-side audio bridge server. It accepts connections from
// container shims, captures audio from the host microphone, and streams
// raw PCM chunks over the connection.
type Server struct {
	capturer Capturer
	listener net.Listener
	portFile string
	sockPath string
	mu       sync.Mutex
	closed   bool
}

// NewServer creates a new Server backed by the given Capturer.
func NewServer(capturer Capturer) *Server {
	return &Server{capturer: capturer}
}

// Start starts the server, creating a Unix socket at sockPath.
// Use this on Linux where bind-mounted Unix sockets work across namespaces.
func (s *Server) Start(sockPath string) error {
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listening on audio socket: %w", err)
	}
	if err := os.Chmod(sockPath, 0o666); err != nil {
		ln.Close() //nolint:errcheck
		return fmt.Errorf("setting audio socket permissions: %w", err)
	}
	s.sockPath = sockPath
	s.listener = ln
	go s.acceptLoop()
	return nil
}

// StartTCP starts the server on TCP 127.0.0.1:0 (OS-assigned port) and writes
// the port number to a file named audio.port in dir. Container-side shims
// read this file and connect via host.docker.internal:<port>.
// Use this on macOS where Unix sockets cannot cross the VM boundary.
func (s *Server) StartTCP(dir string) (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listening on audio TCP: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s.listener = ln

	portFile := filepath.Join(dir, PortFileName)
	if err := os.WriteFile(portFile, []byte(strconv.Itoa(port)), 0o644); err != nil {
		ln.Close() //nolint:errcheck
		return 0, fmt.Errorf("writing audio port file: %w", err)
	}
	s.portFile = portFile

	go s.acceptLoop()
	return port, nil
}

// Close shuts down the server and removes the socket/port files.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var closeErr error
	if s.listener != nil {
		closeErr = s.listener.Close()
	}
	if s.sockPath != "" {
		os.Remove(s.sockPath) //nolint:errcheck
	}
	if s.portFile != "" {
		os.Remove(s.portFile) //nolint:errcheck
	}
	return closeErr
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			slog.Debug("audio server accept error", "error", err)
			continue
		}
		s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	cmdBuf := make([]byte, len(CmdStart))
	n, err := io.ReadFull(conn, cmdBuf)
	if err != nil || string(cmdBuf[:n]) != CmdStart {
		slog.Debug("audio server: invalid start command", "got", string(cmdBuf[:n]))
		return
	}

	if err := s.capturer.Start(); err != nil {
		slog.Debug("audio server: capturer start failed", "error", err)
		errMsg := append([]byte{ErrByte}, []byte(err.Error())...)
		conn.Write(errMsg) //nolint:errcheck
		return
	}

	stopCh := make(chan struct{}, 1)
	go func() {
		buf := make([]byte, len(CmdStop))
		for {
			n, err := conn.Read(buf)
			if err != nil {
				close(stopCh)
				return
			}
			if string(buf[:n]) == CmdStop {
				close(stopCh)
				return
			}
		}
	}()

	chunk := make([]byte, ChunkSize)
	for {
		select {
		case <-stopCh:
			s.capturer.Stop() //nolint:errcheck
			return
		default:
		}

		n, err := s.capturer.Read(chunk)
		if err != nil {
			s.capturer.Stop() //nolint:errcheck
			return
		}
		if _, err := conn.Write(chunk[:n]); err != nil {
			s.capturer.Stop() //nolint:errcheck
			return
		}
	}
}
