// Command analyze reads the persisted trade-results log
// (data/results.jsonl, written by internal/tracker) and reports actual
// win rate broken down by signal name, market type, and confidence
// bucket. Run it after the collector has accumulated enough trades to
// be meaningful (a few hundred, ideally) — with only a handful of
// trades the numbers are noise, not signal.
//
// Usage:
//
//	go run cmd/analyze/main.go [path-to-results.jsonl]
//
// Defaults to data/results.jsonl if no path is given.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"otc-predictor/pkg/types"
)

type bucket struct {
	wins, losses int
	pl           float64
}

func (b bucket) total() int       { return b.wins + b.losses }
func (b bucket) winRate() float64 {
	if b.total() == 0 {
		return 0
	}
	return float64(b.wins) / float64(b.total()) * 100
}

func main() {
	path := "data/results.jsonl"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't open %s: %v\n", path, err)
		fmt.Fprintln(os.Stderr, "(No trades logged yet — this file is written as predictions resolve. Let the collector run a while first.)")
		os.Exit(1)
	}
	defer f.Close()

	bySignal := map[string]*bucket{}
	byMarketType := map[string]*bucket{}
	byConfidence := map[string]*bucket{}
	overall := &bucket{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	count := 0
	for scanner.Scan() {
		var r types.TradeResult
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		count++

		add(overall, r)

		mt := r.MarketType
		if mt == "" {
			mt = "unknown"
		}
		if byMarketType[mt] == nil {
			byMarketType[mt] = &bucket{}
		}
		add(byMarketType[mt], r)

		for _, sig := range r.Signals {
			if bySignal[sig] == nil {
				bySignal[sig] = &bucket{}
			}
			add(bySignal[sig], r)
		}

		cb := confidenceBucket(r.Confidence)
		if byConfidence[cb] == nil {
			byConfidence[cb] = &bucket{}
		}
		add(byConfidence[cb], r)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	if count == 0 {
		fmt.Println("No completed trades in the log yet.")
		return
	}

	fmt.Printf("=== %d trades analyzed ===\n\n", count)

	fmt.Printf("OVERALL: %d wins / %d losses  (%.1f%% win rate, $%.2f P/L)\n\n",
		overall.wins, overall.losses, overall.winRate(), overall.pl)

	if count < 200 {
		fmt.Println("⚠️  Under 200 trades — treat every number below as a rough hint, not a conclusion.")
		fmt.Println("   Small samples produce misleadingly extreme win rates in both directions.")
		fmt.Println()
	}

	printTable("BY SIGNAL (which strategy contributed to the winning direction)", bySignal)
	printTable("BY MARKET TYPE", byMarketType)
	printTable("BY CONFIDENCE BUCKET", byConfidence)

	fmt.Println("How to read this: a signal with a high trade count and win rate meaningfully")
	fmt.Println("above 50% (accounting for the ~15% edge needed to beat an 85% payout, i.e.")
	fmt.Println("above ~54.1%) is worth keeping. One with a low or ~50% win rate over enough")
	fmt.Println("trades is dead weight — its hardcoded confidence value in the strategy code")
	fmt.Println("doesn't match reality and should be lowered or removed.")
}

func add(b *bucket, r types.TradeResult) {
	if r.Won {
		b.wins++
	} else {
		b.losses++
	}
	b.pl += r.ProfitLoss
}

func confidenceBucket(c float64) string {
	switch {
	case c < 0.55:
		return "50-55%"
	case c < 0.60:
		return "55-60%"
	case c < 0.65:
		return "60-65%"
	case c < 0.70:
		return "65-70%"
	case c < 0.80:
		return "70-80%"
	default:
		return "80%+"
	}
}

func printTable(title string, m map[string]*bucket) {
	fmt.Printf("--- %s ---\n", title)
	if len(m) == 0 {
		fmt.Println("(no data)")
		fmt.Println()
		return
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return m[keys[i]].total() > m[keys[j]].total()
	})

	fmt.Printf("%-30s %8s %8s %10s %12s\n", "Name", "Trades", "WinRate", "P/L", "")
	for _, k := range keys {
		b := m[k]
		flag := ""
		if b.total() >= 30 && b.winRate() < 51 {
			flag = "⚠️  weak"
		}
		fmt.Printf("%-30s %8d %7.1f%% %9.2f  %s\n", k, b.total(), b.winRate(), b.pl, flag)
	}
	fmt.Println()
}
