// This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
// Copyright (c) 2026 Uniforge GmbH. All rights reserved.

// Command claustrod is the background daemon that pauses idle claustro sandboxes.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/daemon"
)

func main() {
	root := &cobra.Command{
		Use:           "claustrod",
		Short:         "Background daemon for claustro: pauses idle sandboxes",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the claustrod poll loop",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			return daemon.Run(ctx)
		},
	})
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "claustrod:", err)
		os.Exit(1)
	}
}
