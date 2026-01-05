package main

import (
	"fmt"
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

type Config struct {
	SwitchIP string
	Cookie   string
	Listen   string
}

type Exporter struct {
	mu sync.Mutex
	cfg *Config
}

func NewExporter(cfg *Config) prometheus.Collector {
	return &Exporter{cfg: cfg}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("keeplink_port_tx_good_total", "TX good packets", []string{"port"}, nil)
	ch <- prometheus.NewDesc("keeplink_port_rx_good_total", "RX good packets", []string{"port"}, nil)
	ch <- prometheus.NewDesc("keeplink_port_link_up", "Link status (1=up)", []string{"port"}, nil)
	ch <- prometheus.NewDesc("keeplink_port_speed_config_mbps", "Config speed Mbps", []string{"port"}, nil)
	ch <- prometheus.NewDesc("keeplink_port_speed_actual_mbps", "Actual speed Mbps", []string{"port"}, nil)
	ch <- prometheus.NewDesc("keeplink_port_speed_duplex", "Duplex (1=Full)", []string{"port"}, nil)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	stats := scrapeAll(e.cfg.SwitchIP, e.cfg.Cookie)
	for _, stat := range stats {
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc("keeplink_port_tx_good_total", "TX good", []string{"port"}, nil), prometheus.CounterValue, stat.TxGood, stat.Port)
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc("keeplink_port_rx_good_total", "RX good", []string{"port"}, nil), prometheus.CounterValue, stat.RxGood, stat.Port)
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc("keeplink_port_link_up", "Link up", []string{"port"}, nil), prometheus.GaugeValue, stat.LinkUp, stat.Port)
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc("keeplink_port_speed_config_mbps", "Config Mbps", []string{"port"}, nil), prometheus.GaugeValue, stat.ConfigSpeedMbps, stat.Port)
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc("keeplink_port_speed_actual_mbps", "Actual Mbps", []string{"port"}, nil), prometheus.GaugeValue, stat.ActualSpeedMbps, stat.Port)
		ch <- prometheus.MustNewConstMetric(prometheus.NewDesc("keeplink_port_speed_duplex", "Duplex", []string{"port"}, nil), prometheus.GaugeValue, stat.Duplex, stat.Port)
	}
}

type PortStat struct {
	Port              string
	TxGood, RxGood    float64
	LinkUp            float64
	ConfigSpeedMbps   float64
	ActualSpeedMbps   float64
	Duplex            float64
}

func scrapeAll(ip, cookie string) []PortStat {
	stats := scrapeStats(ip, cookie)
	speeds := scrapeSpeeds(ip, cookie)
	
	var all []PortStat
	for i := 0; i < 9; i++ {
		port := strconv.Itoa(i + 1)
		p := PortStat{Port: port}
		
		if i < len(stats) {
			p.TxGood = stats[i].TxGood
			p.RxGood = stats[i].RxGood
			p.LinkUp = stats[i].LinkUp
		}
		if i < len(speeds) {
			p.ConfigSpeedMbps = speeds[i].ConfigSpeedMbps
			p.ActualSpeedMbps = speeds[i].ActualSpeedMbps
			p.Duplex = speeds[i].Duplex
		}
		all = append(all, p)
	}
	return all
}

func scrapeStats(ip, cookie string) []struct{ TxGood, RxGood, LinkUp float64 } {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/port.cgi?page=stats", ip), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", fmt.Sprintf("http://%s/menu.cgi", ip))
	req.Header.Set("Cookie", cookie)
	
	resp, err := client.Do(req)
	if err != nil { return nil }
	defer resp.Body.Close()
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil { return nil }
	
	var stats []struct{ TxGood, RxGood, LinkUp float64 }
	for i := 0; i < 9; i++ {
		linkUp := 0.0
		if strings.Contains(doc.Find(fmt.Sprintf("table tr:nth-child(%d) td:nth-child(3)", i+2)).Text(), "Up") {
			linkUp = 1.0
		}
		txID := fmt.Sprintf("port%d-txgood", i)
		rxID := fmt.Sprintf("port%d-rxgood", i)
		stats = append(stats, struct{ TxGood, RxGood, LinkUp float64 }{
			TxGood: decode64bit(doc.Find("#"+txID).Text()),
			RxGood: decode64bit(doc.Find("#"+rxID).Text()),
			LinkUp: linkUp,
		})
	}
	return stats
}

func scrapeSpeeds(ip, cookie string) []struct{ ConfigSpeedMbps, ActualSpeedMbps float64; Duplex float64 } {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/port.cgi", ip), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", fmt.Sprintf("http://%s/menu.cgi", ip))
	req.Header.Set("Cookie", cookie)
	
	resp, err := client.Do(req)
	if err != nil { return nil }
	defer resp.Body.Close()
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil { return nil }
	
	var speeds []struct{ ConfigSpeedMbps, ActualSpeedMbps float64; Duplex float64 }
	for i := 10; i <= 18; i++ {  // Tabla status: filas 10-18
		row := doc.Find(fmt.Sprintf("table tr:nth-child(%d)", i)).Find("td")
		if row.Length() < 6 { continue }
		
		config := strings.TrimSpace(row.Eq(2).Text())
		actual := strings.TrimSpace(row.Eq(3).Text())
		
		speeds = append(speeds, struct{ ConfigSpeedMbps, ActualSpeedMbps float64; Duplex float64 }{
			ConfigSpeedMbps: parseSpeed(config),
			ActualSpeedMbps: parseSpeed(actual),
			Duplex:          parseDuplex(actual),
		})
	}
	return speeds
}

func parseSpeed(text string) float64 {
	switch strings.TrimSpace(text) {
	case "10M/Half", "10M/Full": return 10
	case "100M/Half", "100M/Full": return 100
	case "1000Full": return 1000
	case "2500Full": return 2500
	case "10G/Full": return 10000
	default: return 0
	}
}

func parseDuplex(text string) float64 {
	return map[bool]float64{strings.Contains(text, "Full"): 1}[true]
}

func decode64bit(raw string) float64 {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 2 { return 0 }
	hi, _ := strconv.ParseUint(parts[0], 10, 64)
	lo, _ := strconv.ParseUint(parts[1], 10, 64)
	return float64(hi*4294967296 + lo)
}

var rootCmd = &cobra.Command{
	Use: "exporter",
	Run: func(cmd *cobra.Command, args []string) {
		if switchIP == "" {
			log.Fatal("use -i <IP>")
		}
		cfg := &Config{SwitchIP: switchIP, Cookie: cookie, Listen: listenAddr}
		prometheus.MustRegister(NewExporter(cfg))
		
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Listening %s (target: %s)", listenAddr, switchIP)
		log.Fatal(http.ListenAndServe(listenAddr, nil))
	},
}

var (
	switchIP   string
	cookie     string
	listenAddr string = ":9101"
)

func init() {
	rootCmd.Flags().StringVarP(&switchIP, "ip", "i", "", "Switch IP")
	rootCmd.Flags().StringVarP(&cookie, "cookie", "c", "", "Cookie")
	rootCmd.Flags().StringVarP(&listenAddr, "listen", "l", listenAddr, "Listen addr")
}

func main() {
	rootCmd.Execute()
}
