package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"
)

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

// PortMetrics contains all metrics for a port
type PortMetrics struct {
	TxGood          float64
	RxGood          float64
	LinkUp          float64
	ConfigSpeedMbps float64
	ActualSpeedMbps float64
	Duplex          float64
}

var ErrOutOfBoundsMetrics = errors.New("unexpected length of stats slice")

// scrape speed & stats for a switch
func (e *Exporter) scrape(
	ctx context.Context,
	sw *Switch,
) ([]PortMetrics, error) {
	// Scrape both endpoints concurrently
	metrics := make([]PortMetrics, 9)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		stats, err := e.scrapeStats(ctx, sw)
		if err != nil {
			slog.ErrorContext(ctx, "failed to scrape stats", "switch", sw.addr, "err", err)

			return err
		}

		if len(stats) != 9 {
			slog.ErrorContext(ctx, "unexpected port statistics length", "len", len(stats))

			return ErrOutOfBoundsMetrics
		}

		for s := range stats {
			metrics[s].LinkUp = stats[s].LinkUp
			metrics[s].RxGood = stats[s].RxGood
			metrics[s].TxGood = stats[s].TxGood
		}

		return nil
	})

	eg.Go(func() error {
		speeds, err := e.scrapeSpeeds(ctx, sw)
		if err != nil {
			slog.ErrorContext(ctx, "failed to scrape stats", "switch", sw.addr, "err", err)

			return err
		}

		if len(speeds) != 9 {
			slog.ErrorContext(ctx, "unexpected port speed length", "len", len(speeds))

			return ErrOutOfBoundsMetrics
		}

		for s := range speeds {
			metrics[s].ActualSpeedMbps = speeds[s].ActualSpeedMbps
			metrics[s].ConfigSpeedMbps = speeds[s].ConfigSpeedMbps
			metrics[s].Duplex = speeds[s].Duplex
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		slog.ErrorContext(ctx, "failed to scrape metrics", "err", err)

		return nil, err
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
func (e *Exporter) scrapeStats(
	ctx context.Context,
	sw *Switch,
) ([]statsData, error) {
	body, err := sw.fetch(ctx, "/port.cgi?page=stats")
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch ports stats page", "err", err)

		return nil, err
	}

	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stats page: %w", err)
	}

	stats := make([]statsData, 9)

	// Parse TX/RX from elements with IDs port0-txgood, port0-rxgood, etc.
	for portID := range 9 {
		txIDSelector := fmt.Sprintf("#port%d-txgood", portID)
		rxIDSelector := fmt.Sprintf("#port%d-rxgood", portID)

		txText := strings.TrimSpace(doc.Find(txIDSelector).Text())
		rxText := strings.TrimSpace(doc.Find(rxIDSelector).Text())

		txGood := decode64bit(txText)
		rxGood := decode64bit(rxText)

		// Parse link status from table
		linkUp := 0.0
		tableRow := doc.Find(fmt.Sprintf("table tr:nth-child(%d) td:nth-child(3)", portID+2))
		if tableRow.Length() > 0 && strings.Contains(tableRow.Text(), "Up") {
			linkUp = 1.0
		}

		stats[portID] = statsData{
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

var (
	ErrStatusTableMissing = errors.New("status table not found")
)

// scrapeSpeeds retrieves speed and duplex info from /port.cgi
func (e *Exporter) scrapeSpeeds(
	ctx context.Context,
	sw *Switch,
) ([]speedData, error) {
	body, err := sw.fetch(ctx, "/port.cgi")
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch ports page", "err", err)

		return nil, err
	}

	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse speed page", "err", err)

		return nil, err
	}

	speeds := make([]speedData, 9)

	// Find the status table (the one with Port, State, Config, Actual headers)
	// This table shows current port status
	tables := doc.Find("table")
	var statusTable *goquery.Selection

	// Iterate through tables to find the status table
	tables.Each(func(idx int, table *goquery.Selection) {
		// Look for table with "Config" and "Actual" headers in Speed/Duplex section
		headers := table.Find("th")
		for i := range headers.Length() {
			headerText := strings.TrimSpace(headers.Eq(i).Text())
			if strings.Contains(headerText, "Config") || strings.Contains(headerText, "Actual") {
				statusTable = table

				return
			}
		}
	})

	if statusTable == nil {
		return nil, ErrStatusTableMissing
	}

	// Parse rows in the status table
	// Structure: Port | State | Config | Actual | Flow Config | Flow Actual
	//           col0   col1    col2     col3     col4         col5
	rows := statusTable.Find("tr")

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

		if portIdx > 8 || portIdx < 0 {
			slog.ErrorContext(ctx, "invalid port index decoded", "id", portIdx)

			return
		}

		// Column 2: Config speed
		configText := strings.TrimSpace(cols.Eq(2).Text())
		// Column 3: Actual speed/duplex
		actualText := strings.TrimSpace(cols.Eq(3).Text())

		speeds[portIdx] = speedData{
			ConfigSpeedMbps: parseSpeed(configText),
			ActualSpeedMbps: parseSpeed(actualText),
			Duplex:          parseDuplex(actualText),
		}
	})

	return speeds, nil
}
