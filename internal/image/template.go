// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package image

import (
	"bytes"
	_ "embed"
	"log/slog"
	"text/template"

	"github.com/uniforgeai/claustro/internal/config"
)

type templateData struct {
	Go, Rust, Python             bool
	DevTools, BuildTools         bool
	MCPFilesystem, MCPMemory, MCPFetch bool
	VoiceMode                    bool
}

//go:embed Dockerfile.tmpl
var dockerfileTemplate string

var parsedDockerfileTemplate = template.Must(template.New("Dockerfile").Parse(dockerfileTemplate))

// RenderDockerfile renders the Dockerfile template using the given ImageBuildConfig.
func RenderDockerfile(cfg *config.ImageBuildConfig) (string, error) {
	python := cfg.IsLanguageEnabled("python")
	data := templateData{
		Go:            cfg.IsLanguageEnabled("go"),
		Rust:          cfg.IsLanguageEnabled("rust"),
		Python:        python,
		DevTools:      cfg.IsToolGroupEnabled("dev"),
		BuildTools:    cfg.IsToolGroupEnabled("build"),
		MCPFilesystem: cfg.IsMCPServerEnabled("filesystem"),
		MCPMemory:     cfg.IsMCPServerEnabled("memory"),
		// MCPFetch requires Python (pip3); skip it when Python is disabled.
		MCPFetch:  cfg.IsMCPServerEnabled("fetch") && python,
		VoiceMode: cfg.IsToolGroupEnabled("voice"),
	}

	slog.Info("rendering Dockerfile from template",
		"go", data.Go, "rust", data.Rust, "python", data.Python,
		"devTools", data.DevTools, "buildTools", data.BuildTools,
		"mcpFilesystem", data.MCPFilesystem, "mcpMemory", data.MCPMemory, "mcpFetch", data.MCPFetch,
		"voiceMode", data.VoiceMode,
	)

	var buf bytes.Buffer
	if err := parsedDockerfileTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
