package main

import (
	"fmt"
	"time"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/engine"
	"github.com/devblac/watch-tower/internal/sink"
	"github.com/devblac/watch-tower/internal/source/algorand"
	"github.com/devblac/watch-tower/internal/source/evm"
	"github.com/devblac/watch-tower/internal/storage"
	"github.com/spf13/cobra"
)

var (
	flagOnce   bool
	flagDryRun bool
	flagFrom   uint64
	flagTo     uint64
)

func init() {
	runCmd.Flags().BoolVar(&flagOnce, "once", false, "Process one tick and exit")
	runCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Do not send to sinks")
	runCmd.Flags().Uint64Var(&flagFrom, "from", 0, "Start from height/round override")
	runCmd.Flags().Uint64Var(&flagTo, "to", 0, "Stop at height/round (inclusive)")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run watch-tower pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		store, err := storage.Open(cfg.Global.DBPath)
		if err != nil {
			return fmt.Errorf("open storage: %w", err)
		}
		defer store.Close()

		evmScanners := map[string]*evm.Scanner{}
		algoScanners := map[string]*algorand.Scanner{}
		for _, src := range cfg.Sources {
			switch src.Type {
			case "evm":
				if flagFrom > 0 {
					src.StartBlock = fmt.Sprintf("%d", flagFrom)
				}
				cli, err := evm.NewRPCClient(src.RPCURL)
				if err != nil {
					return err
				}
				abis, _ := evm.LoadABIs(src.ABIDirs)
				confirmations := cfg.Global.Confirmations["evm"]
				sc, err := evm.NewScanner(cli, store, src, confirmations, abis, cfg.Rules)
				if err != nil {
					return err
				}
				evmScanners[src.ID] = sc
			case "algorand":
				if flagFrom > 0 {
					src.StartRound = fmt.Sprintf("%d", flagFrom)
				}
				cli, err := algorand.NewAlgodClient(src.AlgodURL)
				if err != nil {
					return err
				}
				confirmations := cfg.Global.Confirmations["algorand"]
				sc, err := algorand.NewScanner(cli, store, src, confirmations, cfg.Rules)
				if err != nil {
					return err
				}
				algoScanners[src.ID] = sc
			}
		}

		sinks := map[string]sink.Sender{}
		for _, s := range cfg.Sinks {
			switch s.Type {
			case "slack":
				sender, err := sink.NewSlackSender(s.WebhookURL, s.Template)
				if err != nil {
					return err
				}
				sinks[s.ID] = sender
			case "teams":
				sender, err := sink.NewTeamsSender(s.WebhookURL, s.Template)
				if err != nil {
					return err
				}
				sinks[s.ID] = sender
			case "webhook":
				sender, err := sink.NewWebhookSender(s.URL, s.Method, s.Template, nil)
				if err != nil {
					return err
				}
				sinks[s.ID] = sender
			default:
				continue
			}
		}

		runner, err := engine.NewRunner(store, cfg, evmScanners, algoScanners, sinks, flagDryRun, flagFrom, flagTo)
		if err != nil {
			return err
		}

		for {
			if err := runner.RunOnce(ctx); err != nil {
				return err
			}
			if flagOnce {
				break
			}
			time.Sleep(1 * time.Second)
		}
		return nil
	},
}
