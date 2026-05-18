package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

// Config holds exporter configuration
type Config struct {
	SwitchIP string
	Cookie   string
	Username string
	Password string
	Listen   string
	Timeout  time.Duration
}

// PortMetrics contains all metrics for a port
type PortMetrics struct {
	Port            string
	TxGood          float64
	RxGood          float64
	LinkUp          float64
	ConfigSpeedMbps float64
	ActualSpeedMbps float64
	Duplex          float64
}

// Exporter implements prometheus.Collector
type Exporter struct {
	mu     sync.RWMutex
	cfg    *Config
	logger *log.Logger

	// HTTP client reused across scrapes
	client *http.Client

	// Cached descriptors (created once, reused)
	portTxGoodTotal     *prometheus.Desc
	portRxGoodTotal     *prometheus.Desc
	portLinkUp          *prometheus.Desc
	portSpeedConfigMbps *prometheus.Desc
	portSpeedActualMbps *prometheus.Desc
	portSpeedDuplex     *prometheus.Desc
	scrapeLastTimestamp *prometheus.Desc
	scrapeDurationMs    *prometheus.Desc
	scrapeErrors        *prometheus.Desc

	// Metrics cache
	lastMetrics    []PortMetrics
	lastScrapeTime time.Time
	lastScrapeErr  error
	lastScrapeDur  time.Duration
}

var (
	ErrNoCredentials = errors.New("A cookie or password & username is required")
)

// NewExporter creates and initializes a new Exporter
func NewExporter(cfg *Config, logger *log.Logger) (*Exporter, error) {
	if cfg.SwitchIP == "" {
		return nil, fmt.Errorf("SwitchIP is required")
	}

	if cfg.Cookie == "" {
		if cfg.Username == "" || cfg.Password == "" {
			return nil, ErrNoCredentials
		}

		sum := md5.Sum([]byte(cfg.Username + cfg.Password))
		cfg.Cookie = "admin=" + hex.EncodeToString(sum[:])
	} else if cfg.Username != "" && cfg.Password != "" {
		logger.Println("a cookie takes precedence over username/password")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	e := &Exporter{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{Timeout: cfg.Timeout},

		// Initialize descriptors with proper names following Prometheus conventions
		portTxGoodTotal: prometheus.NewDesc(
			"keeplink_port_tx_good_packets_total",
			"Total transmitted packets on port",
			[]string{"port"},
			nil,
		),
		portRxGoodTotal: prometheus.NewDesc(
			"keeplink_port_rx_good_packets_total",
			"Total received packets on port",
			[]string{"port"},
			nil,
		),
		portLinkUp: prometheus.NewDesc(
			"keeplink_port_link_up",
			"Port link status (1=up, 0=down)",
			[]string{"port"},
			nil,
		),
		portSpeedConfigMbps: prometheus.NewDesc(
			"keeplink_port_speed_config_mbps",
			"Configured port speed in Mbps",
			[]string{"port"},
			nil,
		),
		portSpeedActualMbps: prometheus.NewDesc(
			"keeplink_port_speed_actual_mbps",
			"Actual port speed in Mbps",
			[]string{"port"},
			nil,
		),
		portSpeedDuplex: prometheus.NewDesc(
			"keeplink_port_speed_duplex",
			"Port duplex mode (1=Full, 0=Half)",
			[]string{"port"},
			nil,
		),
		scrapeLastTimestamp: prometheus.NewDesc(
			"keeplink_scrape_last_timestamp_seconds",
			"Timestamp of last successful scrape",
			nil,
			nil,
		),
		scrapeDurationMs: prometheus.NewDesc(
			"keeplink_scrape_duration_milliseconds",
			"Duration of last scrape in milliseconds",
			nil,
			nil,
		),
		scrapeErrors: prometheus.NewDesc(
			"keeplink_scrape_errors_total",
			"Total number of scrape errors",
			nil,
			nil,
		),
	}

	return e, nil
}

// Describe implements prometheus.Collector.Describe()
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.portTxGoodTotal
	ch <- e.portRxGoodTotal
	ch <- e.portLinkUp
	ch <- e.portSpeedConfigMbps
	ch <- e.portSpeedActualMbps
	ch <- e.portSpeedDuplex
	ch <- e.scrapeLastTimestamp
	ch <- e.scrapeDurationMs
	ch <- e.scrapeErrors
}

// Collect implements prometheus.Collector.Collect()
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Perform scrape with timing
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Timeout)
	defer cancel()

	metrics, err := e.scrapeMetrics(ctx)
	duration := time.Since(start)

	// Track scrape errors
	scrapeErrorCount := 0.0
	if err != nil {
		e.logger.Printf("ERROR: Scrape failed: %v", err)
		e.lastScrapeErr = err
		scrapeErrorCount = 1.0
	} else {
		e.lastMetrics = metrics
		e.lastScrapeTime = time.Now()
		e.lastScrapeErr = nil
	}
	e.lastScrapeDur = duration

	// Report port metrics
	for _, m := range metrics {
		ch <- prometheus.MustNewConstMetric(
			e.portTxGoodTotal,
			prometheus.CounterValue,
			m.TxGood,
			m.Port,
		)
		ch <- prometheus.MustNewConstMetric(
			e.portRxGoodTotal,
			prometheus.CounterValue,
			m.RxGood,
			m.Port,
		)
		ch <- prometheus.MustNewConstMetric(
			e.portLinkUp,
			prometheus.GaugeValue,
			m.LinkUp,
			m.Port,
		)
		ch <- prometheus.MustNewConstMetric(
			e.portSpeedConfigMbps,
			prometheus.GaugeValue,
			m.ConfigSpeedMbps,
			m.Port,
		)
		ch <- prometheus.MustNewConstMetric(
			e.portSpeedActualMbps,
			prometheus.GaugeValue,
			m.ActualSpeedMbps,
			m.Port,
		)
		ch <- prometheus.MustNewConstMetric(
			e.portSpeedDuplex,
			prometheus.GaugeValue,
			m.Duplex,
			m.Port,
		)
	}

	// Report scrape metadata
	ch <- prometheus.MustNewConstMetric(
		e.scrapeLastTimestamp,
		prometheus.GaugeValue,
		float64(e.lastScrapeTime.Unix()),
	)
	ch <- prometheus.MustNewConstMetric(
		e.scrapeDurationMs,
		prometheus.GaugeValue,
		duration.Seconds()*1000,
	)
	ch <- prometheus.MustNewConstMetric(
		e.scrapeErrors,
		prometheus.CounterValue,
		scrapeErrorCount,
	)
}

// scrapeMetrics combines stats and speed data from the switch
func (e *Exporter) scrapeMetrics(ctx context.Context) ([]PortMetrics, error) {
	// Scrape both endpoints in parallel
	statsChan := make(chan map[int]statsData, 1)
	speedChan := make(chan map[int]speedData, 1)
	errChan := make(chan error, 2)

	go func() {
		stats, err := e.scrapeStats(ctx)
		if err != nil {
			errChan <- err
			return
		}
		statsChan <- stats
	}()

	go func() {
		speeds, err := e.scrapeSpeeds(ctx)
		if err != nil {
			errChan <- err
			return
		}
		speedChan <- speeds
	}()

	// Wait for both scrapes with timeout
	var stats map[int]statsData
	var speeds map[int]speedData
	errCount := 0

	for i := 0; i < 2; i++ {
		select {
		case s := <-statsChan:
			stats = s
		case sp := <-speedChan:
			speeds = sp
		case err := <-errChan:
			if err != nil {
				e.logger.Printf("ERROR: Parallel scrape failed: %v", err)
				errCount++
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("scrape context cancelled: %w", ctx.Err())
		}
	}

	if errCount > 0 {
		return nil, fmt.Errorf("scrape encountered %d errors", errCount)
	}

	// Combine stats and speeds into unified metrics
	metrics := make([]PortMetrics, 0, 9)
	for i := 0; i < 9; i++ {
		port := strconv.Itoa(i + 1)
		m := PortMetrics{Port: port}

		if s, ok := stats[i]; ok {
			m.TxGood = s.TxGood
			m.RxGood = s.RxGood
			m.LinkUp = s.LinkUp
		}

		if sp, ok := speeds[i]; ok {
			m.ConfigSpeedMbps = sp.ConfigSpeedMbps
			m.ActualSpeedMbps = sp.ActualSpeedMbps
			m.Duplex = sp.Duplex
		}

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// statsData holds parsed statistics from stats page
type statsData struct {
	TxGood float64
	RxGood float64
	LinkUp float64
}

// scrapeStats retrieves TX/RX/Link status from /port.cgi?page=stats
func (e *Exporter) scrapeStats(ctx context.Context) (map[int]statsData, error) {
	url := fmt.Sprintf("http://%s/port.cgi?page=stats", e.cfg.SwitchIP)
	body, err := e.fetchPage(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stats page: %w", err)
	}
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stats page: %w", err)
	}

	stats := make(map[int]statsData)

	// Parse TX/RX from elements with IDs port0-txgood, port0-rxgood, etc.
	for i := 0; i < 9; i++ {
		portID := strconv.Itoa(i)
		txID := fmt.Sprintf("port%s-txgood", portID)
		rxID := fmt.Sprintf("port%s-rxgood", portID)

		txText := strings.TrimSpace(doc.Find("#" + txID).Text())
		rxText := strings.TrimSpace(doc.Find("#" + rxID).Text())

		txGood := decode64bit(txText)
		rxGood := decode64bit(rxText)

		// Parse link status from table
		linkUp := 0.0
		tableRow := doc.Find(fmt.Sprintf("table tr:nth-child(%d) td:nth-child(3)", i+2))
		if tableRow.Length() > 0 && strings.Contains(tableRow.Text(), "Up") {
			linkUp = 1.0
		}

		stats[i] = statsData{
			TxGood: txGood,
			RxGood: rxGood,
			LinkUp: linkUp,
		}

		// if txGood > 0 || rxGood > 0 {
		// 	e.logger.Printf("DEBUG: Port %d - TX: %.0f, RX: %.0f, Link: %.0f", i+1, txGood, rxGood, linkUp)
		// }
	}

	return stats, nil
}

// speedData holds parsed speed configuration
type speedData struct {
	ConfigSpeedMbps float64
	ActualSpeedMbps float64
	Duplex          float64
}

// scrapeSpeeds retrieves speed and duplex info from /port.cgi
func (e *Exporter) scrapeSpeeds(ctx context.Context) (map[int]speedData, error) {
	url := fmt.Sprintf("http://%s/port.cgi", e.cfg.SwitchIP)
	body, err := e.fetchPage(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch speed page: %w", err)
	}
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse speed page: %w", err)
	}

	speeds := make(map[int]speedData)

	// Find the status table (the one with Port, State, Config, Actual headers)
	// This table shows current port status
	tables := doc.Find("table")
	var statusTable *goquery.Selection

	// Iterate through tables to find the status table
	tables.Each(func(idx int, table *goquery.Selection) {
		// Look for table with "Config" and "Actual" headers in Speed/Duplex section
		headers := table.Find("th")
		for i := 0; i < headers.Length(); i++ {
			headerText := strings.TrimSpace(headers.Eq(i).Text())
			if strings.Contains(headerText, "Config") || strings.Contains(headerText, "Actual") {
				statusTable = table
				return
			}
		}
	})

	if statusTable == nil {
		return nil, fmt.Errorf("status table not found")
	}

	// Parse rows in the status table
	// Structure: Port | State | Config | Actual | Flow Config | Flow Actual
	//           col0   col1    col2     col3     col4         col5
	rows := statusTable.Find("tr")
	portIndex := 0

	rows.Each(func(idx int, row *goquery.Selection) {
		cols := row.Find("td")
		if cols.Length() < 4 {
			// Skip header rows and rows without enough columns
			return
		}

		portText := strings.TrimSpace(cols.Eq(0).Text())
		// Skip if not a port row (e.g., "Port 1")
		if !strings.Contains(portText, "Port") {
			return
		}

		// Extract port number from "Port N"
		parts := strings.Fields(portText)
		if len(parts) < 2 {
			return
		}
		portNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}
		portIdx := portNum - 1

		// Column 2: Config speed
		configText := strings.TrimSpace(cols.Eq(2).Text())
		// Column 3: Actual speed/duplex
		actualText := strings.TrimSpace(cols.Eq(3).Text())

		// e.logger.Printf("DEBUG: Port %d - Config: '%s', Actual: '%s'", portNum, configText, actualText)

		speeds[portIdx] = speedData{
			ConfigSpeedMbps: parseSpeed(configText),
			ActualSpeedMbps: parseSpeed(actualText),
			Duplex:          parseDuplex(actualText),
		}

		portIndex++
	})

	if portIndex == 0 {
		return nil, fmt.Errorf("no port data found in status table")
	}

	return speeds, nil
}

// fetchPage performs an HTTP GET request with proper headers
func (e *Exporter) fetchPage(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Prometheus Exporter/1.0")
	req.Header.Set("Referer", fmt.Sprintf("http://%s/menu.cgi", e.cfg.SwitchIP))
	req.Header.Set("Cookie", e.cfg.Cookie)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// parseSpeed converts speed strings to Mbps float64
func parseSpeed(text string) float64 {
	text = strings.TrimSpace(text)
	switch text {
	case "10M/Half", "10M/Full":
		return 10
	case "100M/Half", "100M/Full":
		return 100
	case "1000Full", "1000M/Full", "1G/Full":
		return 1000
	case "2500Full", "2.5G/Full":
		return 2500
	case "10G/Full":
		return 10000
	default:
		return 0
	}
}

// parseDuplex returns 1.0 for Full duplex, 0.0 for Half
func parseDuplex(text string) float64 {
	if strings.Contains(strings.ToLower(text), "full") {
		return 1.0
	}
	return 0.0
}

// decode64bit decodes 64-bit counters stored as "high-low" strings
func decode64bit(raw string) float64 {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 2 {
		return 0
	}

	hi, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0
	}

	lo, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0
	}

	// Combine high and low 32-bit parts into 64-bit value
	return float64(hi*4294967296 + lo)
}

// CLI setup with Cobra
var (
	switchIP   string
	cookie     string
	username   string
	password   string
	listenAddr string = ":9101"
	timeout    int    = 10
)

var rootCmd = &cobra.Command{
	Use:   "keeplink-exporter",
	Short: "Prometheus exporter for Keeplink network switches",
	Long: `A Prometheus exporter that scrapes metrics from Keeplink network switches.
Exports port statistics including TX/RX packets, link status, and speed/duplex configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(log.Writer(), "[keeplink-exporter] ", log.LstdFlags|log.Lshortfile)

		if switchIP == "" {
			logger.Fatal("ERROR: --ip flag is required")
		}

		cfg := &Config{
			SwitchIP: switchIP,
			Cookie:   cookie,
			Username: username,
			Password: password,
			Listen:   listenAddr,
			Timeout:  time.Duration(timeout) * time.Second,
		}

		exporter, err := NewExporter(cfg, logger)
		if err != nil {
			logger.Fatalf("ERROR: Failed to create exporter: %v", err)
		}

		if err := prometheus.Register(exporter); err != nil {
			logger.Fatalf("ERROR: Failed to register exporter: %v", err)
		}

		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			exporter.mu.RLock()
			lastErr := exporter.lastScrapeErr
			exporter.mu.RUnlock()

			if lastErr != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(w, "Last scrape failed: %v\n", lastErr)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK\n")
		})

		logger.Printf("Starting Keeplink exporter on %s (target: %s)", listenAddr, switchIP)
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			logger.Fatalf("ERROR: Server failed: %v", err)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&switchIP, "ip", "i", "", "Switch IP address (required)")
	rootCmd.Flags().StringVarP(&cookie, "cookie", "c", "", "HTTP Cookie for authentication")
	rootCmd.Flags().StringVarP(&username, "username", "u", "", "password for HTTP cookie")
	rootCmd.Flags().StringVarP(&password, "password", "p", "", "username for HTTP cookie")
	rootCmd.Flags().StringVarP(&listenAddr, "listen", "l", listenAddr, "Listen address for metrics endpoint")
	rootCmd.Flags().IntVar(&timeout, "timeout", timeout, "HTTP request timeout in seconds")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
