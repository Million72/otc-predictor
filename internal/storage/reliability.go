package storage

// ReliabilityFlag summarizes recent real performance for a symbol,
// derived only from actually closed (TP1_HIT / SL_HIT) outcomes —
// never inflated, never assumed.
type ReliabilityFlag struct {
	Symbol      string  `json:"symbol"`
	RecentTotal int     `json:"recentTotal"`
	RecentWins  int     `json:"recentWins"`
	WinRate     float64 `json:"winRate"`
	Reliable    bool    `json:"reliable"` // true only if enough closed samples AND winRate >= 50%
}

const minSamplesForReliability = 5

// GetReliabilityFlags computes per-symbol reliability from real closed outcomes only.
func (s *Store) GetReliabilityFlags() []ReliabilityFlag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type agg struct{ total, wins int }
	bySymbol := map[string]*agg{}

	for _, o := range s.outcomes {
		if o.Outcome != "TP1_HIT" && o.Outcome != "SL_HIT" {
			continue // ignore pending/expired — only judge on real closed results
		}
		a, ok := bySymbol[o.Symbol]
		if !ok {
			a = &agg{}
			bySymbol[o.Symbol] = a
		}
		a.total++
		if o.Outcome == "TP1_HIT" {
			a.wins++
		}
	}

	flags := make([]ReliabilityFlag, 0, len(bySymbol))
	for symbol, a := range bySymbol {
		winRate := 0.0
		if a.total > 0 {
			winRate = float64(a.wins) / float64(a.total) * 100
		}
		flags = append(flags, ReliabilityFlag{
			Symbol:      symbol,
			RecentTotal: a.total,
			RecentWins:  a.wins,
			WinRate:     winRate,
			Reliable:    a.total >= minSamplesForReliability && winRate >= 50,
		})
	}
	return flags
}
