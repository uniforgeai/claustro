package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/uniforgeai/claustro/internal/config"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate claustro.yaml in the current directory",
		Long:  "Validate checks claustro.yaml for configuration errors and warnings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			cfg, err := config.LoadRaw(dir)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if cfg == nil {
				cmd.Println("no claustro.yaml found")
				return nil
			}

			results := cfg.Validate()
			if len(results) == 0 {
				cmd.Println("claustro.yaml: valid")
				return nil
			}

			errCount := 0
			warnCount := 0
			for _, r := range results {
				switch r.Severity {
				case config.SeverityError:
					errCount++
				case config.SeverityWarning:
					warnCount++
				}
			}

			cmd.Printf("claustro.yaml: %d error(s), %d warning(s)\n", errCount, warnCount)
			for _, r := range results {
				cmd.Printf("  [%s] %s: %s\n", r.Severity, r.Field, r.Message)
			}

			if errCount > 0 {
				return fmt.Errorf("claustro.yaml has %d error(s)", errCount)
			}
			return nil
		},
	}
}
