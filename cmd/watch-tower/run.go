package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/engine"
	"github.com/devblac/watch-tower/internal/health"
	"github.com/devblac/watch-tower/internal/logging"
	"github.com/devblac/watch-tower/internal/metrics"
	"github.com/devblac/watch-tower/internal/sink"
	"github.com/devblac/watch-tower/internal/source/algorand"
	"github.com/devblac/watch-tower/internal/source/evm"
	"github.com/devblac/watch-tower/internal/storage"
	"github.com/spf13/cobra"
)

var (
	flagOnce    bool
	flagDryRun  bool
	flagFrom    uint64
	flagTo      uint64
	flagHealth  string
	flagMetrics string
)

func init() {
	runCmd.Flags().BoolVar(&flagOnce, "once", false, "Process one tick and exit")
	runCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Do not send to sinks")
	runCmd.Flags().Uint64Var(&flagFrom, "from", 0, "Start from height/round override")
	runCmd.Flags().Uint64Var(&flagTo, "to", 0, "Stop at height/round (inclusive)")
	runCmd.Flags().StringVar(&flagHealth, "health", "", "Health check HTTP address (e.g., :8080)")
	runCmd.Flags().StringVar(&flagMetrics, "metrics", "", "Metrics HTTP address (e.g., :9090)")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run watch-tower pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		logLevel := os.Getenv("LOG_LEVEL")
		if logLevel == "" {
			logLevel = "info"
		}
		log := logging.NewWithLevel(logLevel)
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

		evmClients := map[string]evm.BlockClient{}
		algoClients := map[string]algorand.AlgodClient{}
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
				evmClients[src.ID] = cli
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
				algoClients[src.ID] = cli
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

		var mtr *metrics.Metrics
		if flagMetrics != "" {
			mtr = metrics.Init()
			log.Info("metrics enabled", "addr", flagMetrics)
		}

		if flagHealth != "" {
			rpcChecker := health.NewRPCChecker(evmClients, algoClients)
			healthSrv := health.Serve(flagHealth, health.Checker{
				DBPing:  store.Ping,
				RPCPing: rpcChecker.Ping,
			})
			log.Info("health check enabled", "addr", flagHealth)
			defer func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = health.Shutdown(shutdownCtx, healthSrv)
			}()
		}

		if flagMetrics != "" {
			go func() {
				mux := http.NewServeMux()
				mux.Handle("/metrics", metrics.Handler())
				srv := &http.Server{Addr: flagMetrics, Handler: mux}
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error("metrics server error", "error", err)
				}
			}()
		}

		runner, err := engine.NewRunner(store, cfg, evmScanners, algoScanners, sinks, flagDryRun, flagFrom, flagTo)
		if err != nil {
			return err
		}

		for {
			if err := runner.RunOnce(ctx); err != nil {
				if mtr != nil {
					mtr.Errors()
				}
				log.Error("run error", "error", err)
				return err
			}
			if mtr != nil {
				mtr.BlocksProcessed()
			}
			log.Info("tick complete", "dry_run", flagDryRun)
			if flagOnce {
				break
			}
			time.Sleep(1 * time.Second)
		}
		return nil
	},
}
