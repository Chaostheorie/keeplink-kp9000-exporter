package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Switch struct {
	client *http.Client
	cookie string
	addr   string
}

func (sw *Switch) fetch(
	ctx context.Context,
	path string,
) (io.ReadCloser, error) {
	url := "http://" + sw.addr + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Prometheus Exporter/1.0")
	req.Header.Set("Referer", fmt.Sprintf("http://%s/menu.cgi", sw.addr))
	req.Header.Set("Cookie", sw.cookie)

	resp, err := sw.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// Exporter implements [prometheus.Collector]
type Exporter struct {
	portTxGoodTotal     *prometheus.GaugeVec
	portRxGoodTotal     *prometheus.GaugeVec
	portLinkUp          *prometheus.GaugeVec
	portSpeedConfigMbps *prometheus.GaugeVec
	portSpeedActualMbps *prometheus.GaugeVec
	portSpeedDuplex     *prometheus.GaugeVec
	scrapeLastTimestamp *prometheus.GaugeVec
	scrapeDurationMs    *prometheus.GaugeVec
	scrapeErrors        *prometheus.CounterVec
	targets             []*Switch   // scrape targets
	busy                atomic.Bool // marker for currently running scrape
}

var (
	switchLabels = []string{"target"}
	portLabels   = append(switchLabels, "port")
)

func NewExporter(targets []*Switch) *Exporter {
	return &Exporter{
		targets: targets,
		portTxGoodTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "port", Name: "tx_good_packets_total",
			Help: "Total transmitted packets on port",
		}, portLabels),
		portRxGoodTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "port", Name: "rx_good_packets_total",
			Help: "Total received packets on port",
		}, portLabels),
		portLinkUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "port", Name: "link_up",
			Help: "Port link status (1=up, 0=down)",
		}, portLabels),
		portSpeedConfigMbps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "port", Name: "speed_config_mbps",
			Help: "Configured port speed in Mbps",
		}, portLabels),
		portSpeedActualMbps: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "port", Name: "speed_actual_mbps",
			Help: "Actual port speed in Mbps",
		}, portLabels),
		portSpeedDuplex: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "port", Name: "speed_duplex",
			Help: "Port duplex mode (1=Full, 0=Half)",
		}, portLabels),
		scrapeLastTimestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "scrape", Name: "last_timestamp_seconds",
			Help: "Timestamp of last successful scrape",
		}, switchLabels),
		scrapeDurationMs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "keeplink", Subsystem: "scrape", Name: "duration_milliseconds",
			Help: "Duration of last scrape in milliseconds",
		}, switchLabels),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "keeplink", Subsystem: "scrape", Name: "errors_total",
			Help: "Total number of scrape errors",
		}, switchLabels),
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	if e.busy.CompareAndSwap(false, true) {
		e.busy.Store(false)

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer cancel()

		var wg sync.WaitGroup

		for _, sw := range e.targets {
			wg.Go(func() {
				start := time.Now()

				metrics, err := e.scrape(ctx, sw)

				e.scrapeDurationMs.WithLabelValues(sw.addr).Set(float64(
					time.Since(start).Milliseconds(),
				))

				if err != nil {
					slog.Error("failed scrape", "err", err)

					e.scrapeErrors.WithLabelValues(sw.addr).Inc()

					return
				}

				for p, m := range metrics {
					pNum := strconv.FormatInt(int64(p), 10)

					e.portRxGoodTotal.WithLabelValues(sw.addr, pNum).Set(m.RxGood)
					e.portTxGoodTotal.WithLabelValues(sw.addr, pNum).Set(m.TxGood)

					e.portLinkUp.WithLabelValues(sw.addr, pNum).Set(m.LinkUp)

					e.portSpeedActualMbps.WithLabelValues(sw.addr, pNum).Set(m.ActualSpeedMbps)
					e.portSpeedConfigMbps.WithLabelValues(sw.addr, pNum).Set(m.ConfigSpeedMbps)
					e.portSpeedDuplex.WithLabelValues(sw.addr, pNum).Set(m.Duplex)
				}
			})
		}

		wg.Wait()
	}

	e.portRxGoodTotal.Collect(ch)
	e.portTxGoodTotal.Collect(ch)
	e.portLinkUp.Collect(ch)
	e.portSpeedActualMbps.Collect(ch)
	e.portSpeedConfigMbps.Collect(ch)
	e.portSpeedDuplex.Collect(ch)
	e.scrapeLastTimestamp.Collect(ch)
	e.scrapeDurationMs.Collect(ch)
	e.scrapeErrors.Collect(ch)
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.portRxGoodTotal.Describe(ch)
	e.portTxGoodTotal.Describe(ch)
	e.portLinkUp.Describe(ch)
	e.portSpeedActualMbps.Describe(ch)
	e.portSpeedConfigMbps.Describe(ch)
	e.portSpeedDuplex.Describe(ch)
	e.scrapeLastTimestamp.Describe(ch)
	e.scrapeDurationMs.Describe(ch)
	e.scrapeErrors.Describe(ch)
}
