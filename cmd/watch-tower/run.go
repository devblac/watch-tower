package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run watch-tower pipelines (stub)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Implement pipeline run with replay/dry-run/once support.
		fmt.Fprintln(cmd.OutOrStdout(), "run: TODO execute pipelines with replay, dry-run, once.")
		return nil
	},
}
