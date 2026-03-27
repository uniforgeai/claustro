package clipboard

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHandler is a PlatformHandler that returns configurable test data.
type mockHandler struct {
	types     []string
	imageData []byte
	text      string
	typesErr  error
	imageErr  error
	textErr   error
}

func (m *mockHandler) Types() ([]string, error)  { return m.types, m.typesErr }
func (m *mockHandler) ReadImage() ([]byte, error) { return m.imageData, m.imageErr }
func (m *mockHandler) ReadText() (string, error)  { return m.text, m.textErr }

func unixClient(sockPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
			},
		},
	}
}

func TestServer_Types_withImage(t *testing.T) {
	handler := &mockHandler{types: []string{"image/png", "text/plain"}}
	srv := New(handler)

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))
	defer srv.Close()

	resp, err := unixClient(sockPath).Get("http://x/types")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "image/png")
}

func TestServer_Types_noImage(t *testing.T) {
	handler := &mockHandler{types: nil}
	srv := New(handler)

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))
	defer srv.Close()

	resp, err := unixClient(sockPath).Get("http://x/types")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_ImagePNG_present(t *testing.T) {
	pngData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	handler := &mockHandler{imageData: pngData}
	srv := New(handler)

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))
	defer srv.Close()

	resp, err := unixClient(sockPath).Get("http://x/image/png")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, pngData, body)
}

func TestServer_ImagePNG_absent(t *testing.T) {
	handler := &mockHandler{imageData: nil}
	srv := New(handler)

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))
	defer srv.Close()

	resp, err := unixClient(sockPath).Get("http://x/image/png")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_Text_present(t *testing.T) {
	handler := &mockHandler{text: "hello clipboard"}
	srv := New(handler)

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))
	defer srv.Close()

	resp, err := unixClient(sockPath).Get("http://x/text")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "hello clipboard", string(body))
}

func TestServer_Text_absent(t *testing.T) {
	handler := &mockHandler{text: ""}
	srv := New(handler)

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))
	defer srv.Close()

	resp, err := unixClient(sockPath).Get("http://x/text")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestServer_Close_removesSockFile(t *testing.T) {
	srv := New(&mockHandler{})

	sockPath := filepath.Join(t.TempDir(), "clipboard.sock")
	require.NoError(t, srv.Start(sockPath))

	_, err := os.Stat(sockPath)
	require.NoError(t, err, "socket file should exist after Start")

	require.NoError(t, srv.Close())

	_, err = os.Stat(sockPath)
	assert.True(t, os.IsNotExist(err), "socket file should be removed after Close")
}
