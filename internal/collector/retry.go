package collector

import (
	"strings"
	"time"

	"otc-predictor/pkg/types"
)

// FetchCandlesWithRetry attempts the fetch, retrying up to 3 times with
// growing backoff if it fails (handles transient Deriv WS dial/handshake
// hiccups, which have been observed to need more than one short retry).
// "Invalid symbol" errors are permanent (Deriv itself rejected the symbol
// code, not a connection problem), so those are returned immediately
// without wasting retries.
func (d *DerivClient) FetchCandlesWithRetry(symbol string, granularity, count int) ([]types.Candle, error) {
	backoffs := []time.Duration{1500 * time.Millisecond, 3 * time.Second, 6 * time.Second}

	candles, err := d.FetchCandles(symbol, granularity, count)
	if err == nil {
		return candles, nil
	}

	for _, backoff := range backoffs {
		if isPermanentDerivError(err) {
			return nil, err
		}
		time.Sleep(backoff)
		candles, err = d.FetchCandles(symbol, granularity, count)
		if err == nil {
			return candles, nil
		}
	}
	return nil, err
}

func isPermanentDerivError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "deriv error:") && strings.Contains(strings.ToLower(msg), "invalid symbol")
}
