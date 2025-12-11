package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = ""
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "watch-tower %s", version)
		if commit != "" && commit != "none" {
			fmt.Fprintf(out, " commit %s", commit)
		}
		if date != "" {
			fmt.Fprintf(out, " built %s", date)
		}
		fmt.Fprintln(out)
		return nil
	},
}
