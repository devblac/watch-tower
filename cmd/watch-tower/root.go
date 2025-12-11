package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgPath string
	rootCmd = &cobra.Command{
		Use:   "watch-tower",
		Short: "Cross-chain monitoring & alerts CLI (EVM + Algorand)",
	}
)

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "config.yaml", "Path to config file")

	rootCmd.AddCommand(
		versionCmd,
		initCmd,
		validateCmd,
		runCmd,
		stateCmd,
		exportCmd,
	)
}

// Execute runs the root command tree.
func Execute() error {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
