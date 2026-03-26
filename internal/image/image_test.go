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

func TestExtImageName(t *testing.T) {
	tests := []struct {
		project string
		want    string
	}{
		{"myapp", "claustro-myapp:latest"},
		{"my-saas", "claustro-my-saas:latest"},
		{"proj123", "claustro-proj123:latest"},
	}
	for _, tt := range tests {
		t.Run(tt.project, func(t *testing.T) {
			assert.Equal(t, tt.want, ExtImageName(tt.project))
		})
	}
}

func TestExtHash_Determinism(t *testing.T) {
	steps := []string{"apt-get install -y ffmpeg", "pip install black"}
	h1 := extHash(steps)
	h2 := extHash(steps)
	assert.Equal(t, h1, h2)
	assert.NotEmpty(t, h1)
}

func TestExtHash_OrderSensitive(t *testing.T) {
	h1 := extHash([]string{"step-a", "step-b"})
	h2 := extHash([]string{"step-b", "step-a"})
	assert.NotEqual(t, h1, h2)
}

func TestExtHash_DifferentSteps(t *testing.T) {
	h1 := extHash([]string{"apt-get install -y ffmpeg"})
	h2 := extHash([]string{"apt-get install -y curl"})
	assert.NotEqual(t, h1, h2)
}

func TestExtBuildContext_ContainsDockerfile(t *testing.T) {
	steps := []string{"apt-get install -y ffmpeg", "pip install black"}
	ctx, err := extBuildContext(steps)
	require.NoError(t, err)
	assert.NotEmpty(t, ctx)

	// Extract and verify Dockerfile content
	tr := tar.NewReader(bytes.NewReader(ctx))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Name == "Dockerfile" {
			content, err := io.ReadAll(tr)
			require.NoError(t, err)
			df := string(content)
			assert.Contains(t, df, "FROM claustro:latest")
			assert.Contains(t, df, "RUN apt-get install -y ffmpeg")
			assert.Contains(t, df, "RUN pip install black")
			return
		}
	}
	t.Fatal("Dockerfile not found in ext build context")
}

func TestExtBuildContext_StepOrder(t *testing.T) {
	steps := []string{"step-first", "step-second"}
	ctx, err := extBuildContext(steps)
	require.NoError(t, err)

	tr := tar.NewReader(bytes.NewReader(ctx))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Name == "Dockerfile" {
			content, err := io.ReadAll(tr)
			require.NoError(t, err)
			df := string(content)
			firstIdx := strings.Index(df, "step-first")
			secondIdx := strings.Index(df, "step-second")
			assert.Less(t, firstIdx, secondIdx, "step-first should appear before step-second")
			return
		}
	}
	t.Fatal("Dockerfile not found")
}
