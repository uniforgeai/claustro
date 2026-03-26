package image

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildContext_ContainsRequiredFiles(t *testing.T) {
	ctx, err := buildContext()
	require.NoError(t, err)
	assert.NotEmpty(t, ctx)

	files := map[string]bool{}
	tr := tar.NewReader(bytes.NewReader(ctx))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		files[hdr.Name] = true
	}

	assert.True(t, files["Dockerfile"], "expected Dockerfile in build context")
	assert.True(t, files["claustro-init"], "expected claustro-init in build context")
}

func TestBuildContext_InitScriptIsExecutable(t *testing.T) {
	ctx, err := buildContext()
	require.NoError(t, err)

	tr := tar.NewReader(bytes.NewReader(ctx))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Name == "claustro-init" {
			assert.Equal(t, int64(0755), hdr.Mode, "claustro-init should have mode 0755")
			return
		}
	}
	t.Fatal("claustro-init not found in build context")
}

func TestStreamBuildOutput_Success(t *testing.T) {
	lines := `{"stream":"Step 1/5\n"}` + "\n" +
		`{"stream":"Step 2/5\n"}` + "\n"

	var out strings.Builder
	err := streamBuildOutput(strings.NewReader(lines), &out)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Step 1/5")
	assert.Contains(t, out.String(), "Step 2/5")
}

func TestStreamBuildOutput_BuildError(t *testing.T) {
	lines := `{"stream":"Step 1/5\n"}` + "\n" +
		`{"error":"failed to build: some error"}` + "\n"

	var out strings.Builder
	err := streamBuildOutput(strings.NewReader(lines), &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some error")
}

func TestStreamBuildOutput_SkipsMalformedJSON(t *testing.T) {
	lines := "not json\n" +
		`{"stream":"valid line\n"}` + "\n"

	var out strings.Builder
	err := streamBuildOutput(strings.NewReader(lines), &out)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "valid line")
}
