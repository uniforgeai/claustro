// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// rec-shim is a drop-in replacement for SoX's `rec` command inside claustro
// containers. Instead of capturing from a hardware audio device, it connects
// to the claustro audio bridge on the host and receives PCM data via socket.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/uniforgeai/claustro/internal/audio"
)

func main() {
	debug := os.Getenv("CLAUSTRO_AUDIO_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "rec-shim: args=%v\n", os.Args)
	}

	outFile := parseOutputFile(os.Args)
	if outFile == "" {
		fmt.Fprintln(os.Stderr, "rec-shim: no output file specified")
		os.Exit(1)
	}

	sockPath := os.Getenv("CLAUSTRO_AUDIO_SOCK")
	host := os.Getenv("CLAUSTRO_AUDIO_HOST")
	portStr := os.Getenv("CLAUSTRO_AUDIO_PORT")
	if debug {
		fmt.Fprintf(os.Stderr, "rec-shim: sock=%q host=%q port=%q out=%q\n", sockPath, host, portStr, outFile)
	}

	var port int
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rec-shim: invalid CLAUSTRO_AUDIO_PORT: %s\n", portStr)
			os.Exit(1)
		}
	}

	if sockPath == "" && host == "" {
		fmt.Fprintln(os.Stderr, "rec-shim: audio bridge not available — is voice mode enabled in claustro.yaml?")
		os.Exit(1)
	}

	if err := recordToWAV(sockPath, host, port, outFile); err != nil {
		fmt.Fprintf(os.Stderr, "rec-shim: %v\n", err)
		os.Exit(1)
	}
}

// parseOutputFile extracts the output filename from rec-style arguments.
// The output file is the last argument that doesn't start with '-'.
func parseOutputFile(args []string) string {
	for i := len(args) - 1; i >= 1; i-- {
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
	}
	return ""
}

// recordToWAV connects to the audio bridge, streams PCM, and writes a WAV file.
func recordToWAV(sockPath, host string, port int, outFile string) error {
	var conn net.Conn
	var err error

	if sockPath != "" {
		conn, err = net.Dial("unix", sockPath)
	} else {
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	}
	if err != nil {
		return fmt.Errorf("connecting to audio bridge: %w", err)
	}
	defer conn.Close()

	// Send START.
	if _, err := conn.Write([]byte(audio.CmdStart)); err != nil {
		return fmt.Errorf("sending START: %w", err)
	}

	// Check for error response.
	peek := make([]byte, 1)
	n, err := conn.Read(peek)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if n > 0 && peek[0] == audio.ErrByte {
		errMsg, _ := io.ReadAll(conn)
		return fmt.Errorf("audio bridge error: %s", string(errMsg))
	}

	// Set up signal handler for graceful stop.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Collect PCM data. Start with the byte we already peeked.
	var pcmBuf bytes.Buffer
	pcmBuf.Write(peek[:n])

	buf := make([]byte, audio.ChunkSize)
	done := false

	go func() {
		<-sigCh
		conn.Write([]byte(audio.CmdStop)) //nolint:errcheck
		done = true
	}()

	for !done {
		n, err := conn.Read(buf)
		if n > 0 {
			pcmBuf.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	// Write WAV file.
	f, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	pcmData := pcmBuf.Bytes()
	if os.Getenv("CLAUSTRO_AUDIO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "rec-shim: writing WAV to %s (%d bytes PCM)\n", outFile, len(pcmData))
	}
	if err := audio.WriteWAVHeader(f, uint32(len(pcmData))); err != nil {
		return fmt.Errorf("writing WAV header: %w", err)
	}
	if _, err := f.Write(pcmData); err != nil {
		return fmt.Errorf("writing PCM data: %w", err)
	}

	return nil
}
