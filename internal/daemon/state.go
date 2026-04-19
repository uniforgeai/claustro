// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

package daemon

// PollInterval is how often the daemon polls the Docker SDK.
import "time"

const PollInterval = 30 * time.Second
