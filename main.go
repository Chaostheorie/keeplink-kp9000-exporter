package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "keeplink-exporter",
	Short: "Prometheus exporter for Keeplink network switches",
	Long: `A Prometheus exporter that scrapes metrics from Keeplink network switches.
Exports port statistics including TX/RX packets, link status, and speed/duplex configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		scrapeConfigContent, err := os.ReadFile(scrapeConfig)
		if err != nil {
			slog.Error("failed to read scrape config", "err", err, "path", scrapeConfig)

			os.Exit(1)
		}

		var scrapeCfg ScrapeConfig
		if err := yaml.Unmarshal(scrapeConfigContent, &scrapeCfg); err != nil {
			slog.Error("failed to parse scrape config", "err", err, "path", scrapeConfig)

			os.Exit(1)
		}

		if len(scrapeCfg.Switches) == 0 {
			slog.Error("you need to specify at least one switch")

			os.Exit(1)
		}

		switches := make([]*Switch, len(scrapeCfg.Switches))

		for s := range scrapeCfg.Switches {
			sw, err := scrapeCfg.Switches[s].Load()
			if err != nil {
				slog.Error(
					"failed to evaluate switch scrape config",
					"switch", scrapeCfg.Switches[s].SwitchIP, "err", err,
				)

				os.Exit(1)
			}

			switches[s] = sw
		}

		exporter := NewExporter(switches)

		if err := prometheus.Register(exporter); err != nil {
			slog.Error("failed to register switch exporter", "err", err)

			os.Exit(1)
		}

		http.Handle("/metrics", promhttp.Handler())

		slog.Info("Starting Keeplink exporter", "listen", listenAddr)
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			slog.Error("failed to run exporter", "err", err)
		} else {
			slog.Info("exporter is done")
		}
	},
}

var (
	listenAddr   = "127.0.0.1:8192"
	scrapeConfig = "scrape-cfg-keeplink.yml"
	timeout      int
)

func init() {
	rootCmd.Flags().StringVarP(&listenAddr, "listen", "l", listenAddr, "Listen address for metrics endpoint")
	rootCmd.Flags().StringVarP(&scrapeConfig, "scrape-config", "c", scrapeConfig, "scrape config as YAML")
	rootCmd.Flags().IntVar(&timeout, "timeout", timeout, "HTTP request timeout in seconds")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
