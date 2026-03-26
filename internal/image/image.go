// Package image manages the claustro Docker image.
package image

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	_ "embed"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

//go:embed Dockerfile
var dockerfile []byte

//go:embed claustro-init
var initScript []byte

const ImageName = "claustro:latest"

// EnsureBuilt checks whether the claustro image exists and builds it if not.
// Build output is written to w.
func EnsureBuilt(ctx context.Context, cli *client.Client, w io.Writer) error {
	exists, err := imageExists(ctx, cli)
	if err != nil {
		return fmt.Errorf("checking image: %w", err)
	}
	if exists {
		return nil
	}
	slog.Info("building image", "image", ImageName)
	return buildImage(ctx, cli, false, w)
}

// Build forces a full rebuild of the claustro image regardless of whether it exists.
// Build output is written to w.
func Build(ctx context.Context, cli *client.Client, w io.Writer) error {
	slog.Info("rebuilding image", "image", ImageName)
	return buildImage(ctx, cli, true, w)
}

func imageExists(ctx context.Context, cli *client.Client) (bool, error) {
	args := filters.NewArgs(filters.Arg("reference", ImageName))
	images, err := cli.ImageList(ctx, imagetypes.ListOptions{Filters: args})
	if err != nil {
		return false, err
	}
	return len(images) > 0, nil
}

func buildImage(ctx context.Context, cli *client.Client, noCache bool, w io.Writer) error {
	buildCtx, err := buildContext()
	if err != nil {
		return fmt.Errorf("creating build context: %w", err)
	}

	resp, err := cli.ImageBuild(ctx, bytes.NewReader(buildCtx), build.ImageBuildOptions{
		Tags:       []string{ImageName},
		Dockerfile: "Dockerfile",
		Remove:     true,
		NoCache:    noCache,
	})
	if err != nil {
		return fmt.Errorf("starting image build: %w", err)
	}
	defer resp.Body.Close()

	return streamBuildOutput(resp.Body, w)
}

// buildContext creates an in-memory tar archive containing the Dockerfile and init script.
func buildContext() ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	files := []struct {
		name string
		data []byte
		mode int64
	}{
		{"Dockerfile", dockerfile, 0644},
		{"claustro-init", initScript, 0755},
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.name,
			Mode: f.mode,
			Size: int64(len(f.data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type buildMessage struct {
	Stream string `json:"stream"`
	Error  string `json:"error"`
}

func streamBuildOutput(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var msg buildMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if msg.Error != "" {
			return fmt.Errorf("image build failed: %s", msg.Error)
		}
		if msg.Stream != "" {
			fmt.Fprint(w, msg.Stream)
		}
	}
	return scanner.Err()
}
