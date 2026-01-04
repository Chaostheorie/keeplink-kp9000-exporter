package parser

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type PortStats struct {
	Port   string
	TxGood float64
	RxGood float64
	LinkUp float64
}

func Parse(body []byte) ([]PortStats, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	
	var stats []PortStats
	for portID := 0; portID <= 8; portID++ {
		portNum := fmt.Sprintf("%d", portID+1)
		
		txSel := fmt.Sprintf("#port%d-txgood", portID)
		rxSel := fmt.Sprintf("#port%d-rxgood", portID)
		
		txStr := strings.TrimSpace(doc.Find(txSel).Text())
		rxStr := strings.TrimSpace(doc.Find(rxSel).Text())
		
		stats = append(stats, PortStats{
			Port:   portNum,
			TxGood: decodeStat(txStr),
			RxGood: decodeStat(rxStr),
		})
	}
	return stats, nil
}

func decodeStat(s string) float64 {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0
	}
	hi, _ := strconv.ParseUint(parts[0], 10, 64)
	lo, _ := strconv.ParseUint(parts[1], 10, 64)
	return float64(hi<<32 + lo)
}

