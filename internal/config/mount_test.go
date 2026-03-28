package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMount(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		projectRoot string
		want        Mount
		wantErr     string
	}{
		{
			name:        "absolute path",
			raw:         "/host/libs:/workspace/libs",
			projectRoot: "/project",
			want:        Mount{HostPath: "/host/libs", ContainerPath: "/workspace/libs"},
		},
		{
			name:        "relative path resolved against project root",
			raw:         "./libs:/workspace/libs",
			projectRoot: "/project",
			want:        Mount{HostPath: "/project/libs", ContainerPath: "/workspace/libs"},
		},
		{
			name:        "ro mode",
			raw:         "./libs:/workspace/libs:ro",
			projectRoot: "/project",
			want:        Mount{HostPath: "/project/libs", ContainerPath: "/workspace/libs", ReadOnly: true},
		},
		{
			name:        "rw mode explicit",
			raw:         "/host/data:/data:rw",
			projectRoot: "/project",
			want:        Mount{HostPath: "/host/data", ContainerPath: "/data"},
		},
		{
			name:        "missing container path",
			raw:         "/host/only",
			projectRoot: "/project",
			wantErr:     "expected host:container[:mode]",
		},
		{
			name:        "invalid mode",
			raw:         "/a:/b:xyz",
			projectRoot: "/project",
			wantErr:     "invalid mount mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMount(tt.raw, tt.projectRoot)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
