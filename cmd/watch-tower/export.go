package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export alerts or cursors (stub)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Export alerts/cursors as csv/json.
		fmt.Fprintln(cmd.OutOrStdout(), "export: TODO export alerts|cursors to csv|json.")
		return nil
	},
}
