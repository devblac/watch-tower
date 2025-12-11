package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Show cursors and processing lag (stub)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Read cursor state from storage and present lag per source.
		fmt.Fprintln(cmd.OutOrStdout(), "state: TODO display cursors and lag.")
		return nil
	},
}
