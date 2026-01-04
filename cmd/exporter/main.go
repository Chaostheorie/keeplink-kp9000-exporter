package main

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/sergiosanche/keeplink-kp9000-exporter/internal/exporter"
)

var rootCmd = &cobra.Command{
	Use:   "keeplink-exporter",
	Short: "Prometheus exporter for Keeplink KP9000-9XHML-X switch",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := exporter.LoadConfig()
		exp := exporter.New(cfg)
		prometheus.MustRegister(exp)
		
		log.Printf("Keeplink exporter listening on %s (target: %s)", cfg.ListenAddr, cfg.SwitchIP)
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(cfg.ListenAddr, nil))
	},
}

func init() {
	rootCmd.Flags().StringP("switch-ip", "i", "192.168.1.168", "Switch IP address")
	rootCmd.Flags().StringP("listen", "l", ":9100", "Listen address")
	rootCmd.Flags().String("cookie", "", "Session cookie (admin=...)")
	rootCmd.Flags().String("user", "", "Web UI username")
	rootCmd.Flags().String("pass", "", "Web UI password")
}

func main() {
	rootCmd.Execute()
}
