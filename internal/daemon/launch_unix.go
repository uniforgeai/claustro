// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

//go:build unix

package daemon

import "syscall"

func sysprocattrDetach() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
