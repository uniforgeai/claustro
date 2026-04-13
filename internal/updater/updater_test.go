// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectMethod_ReturnsAMethod(t *testing.T) {
	// On CI/test the binary won't be in Homebrew or go/bin, so expect Unknown
	method := DetectMethod()
	assert.Contains(t, []Method{MethodUnknown, MethodHomebrew, MethodGoInstall}, method)
}

func TestUpdate_UnknownMethod_ReturnsError(t *testing.T) {
	_, err := Update(MethodUnknown, "dev")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot auto-update")
	assert.Contains(t, err.Error(), "github.com/uniforgeai/claustro/releases")
}
