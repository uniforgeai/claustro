// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolConstants(t *testing.T) {
	assert.Equal(t, 2, FrameSize, "16-bit mono = 2 bytes per frame")
	assert.Equal(t, 2048, ChunkSize, "1024 frames * 2 bytes")
	assert.Equal(t, 5, len(CmdStart))
	assert.Equal(t, 4, len(CmdStop))
}
