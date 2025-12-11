package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold sample config and project files (stub)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Generate sample config, ABIs, CI workflow, and goreleaser stub.
		fmt.Fprintln(cmd.OutOrStdout(), "init: TODO scaffold config, ABIs, CI workflow, goreleaser.")
		return nil
	},
}
