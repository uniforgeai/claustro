// Package image manages the claustro Docker image.
package image

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

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

// ExtImageName returns the Docker image tag for a project's extension image.
// Format: claustro-{project}:latest
func ExtImageName(project string) string {
	return fmt.Sprintf("claustro-%s:latest", project)
}

// extHash returns a SHA-256 hex digest of the ordered list of RUN steps.
// Used as a change-detection label on extension images.
func extHash(steps []string) string {
	h := sha256.New()
	for _, s := range steps {
		h.Write([]byte(s))
		h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// EnsureExtended builds a project-specific extension image if it does not exist
// or if the image's ext-hash label no longer matches the given steps.
// The extension image is tagged claustro-{project}:latest and layers over claustro:latest.
func EnsureExtended(ctx context.Context, cli *client.Client, project string, steps []string, w io.Writer) error {
	upToDate, err := extImageUpToDate(ctx, cli, project, steps)
	if err != nil {
		return fmt.Errorf("checking extension image: %w", err)
	}
	if upToDate {
		return nil
	}
	slog.Info("building extension image", "image", ExtImageName(project))
	return buildExtended(ctx, cli, project, steps, w)
}

// BuildExtended forces a rebuild of the project extension image regardless of its current state.
func BuildExtended(ctx context.Context, cli *client.Client, project string, steps []string, w io.Writer) error {
	slog.Info("rebuilding extension image", "image", ExtImageName(project))
	return buildExtended(ctx, cli, project, steps, w)
}

func extImageUpToDate(ctx context.Context, cli *client.Client, project string, steps []string) (bool, error) {
	args := filters.NewArgs(filters.Arg("reference", ExtImageName(project)))
	images, err := cli.ImageList(ctx, imagetypes.ListOptions{Filters: args})
	if err != nil {
		return false, fmt.Errorf("listing images: %w", err)
	}
	if len(images) == 0 {
		return false, nil
	}
	return images[0].Labels["claustro.ext-hash"] == extHash(steps), nil
}

func buildExtended(ctx context.Context, cli *client.Client, project string, steps []string, w io.Writer) error {
	buildCtx, err := extBuildContext(steps)
	if err != nil {
		return fmt.Errorf("creating build context: %w", err)
	}

	resp, err := cli.ImageBuild(ctx, bytes.NewReader(buildCtx), build.ImageBuildOptions{
		Tags:       []string{ExtImageName(project)},
		Dockerfile: "Dockerfile",
		Remove:     true,
		Labels:     map[string]string{"claustro.ext-hash": extHash(steps)},
	})
	if err != nil {
		return fmt.Errorf("starting extension image build: %w", err)
	}
	defer resp.Body.Close()

	return streamBuildOutput(resp.Body, w)
}

// extBuildContext creates an in-memory tar archive containing a generated Dockerfile
// that extends claustro:latest with the given RUN steps.
func extBuildContext(steps []string) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("FROM ")
	sb.WriteString(ImageName)
	sb.WriteString("\n")
	for _, step := range steps {
		sb.WriteString("RUN ")
		sb.WriteString(step)
		sb.WriteString("\n")
	}

	content := []byte(sb.String())
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: "Dockerfile",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
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
