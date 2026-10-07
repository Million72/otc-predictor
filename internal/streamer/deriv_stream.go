package streamer

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"otc-predictor/pkg/types"
)

// DerivStreamer maintains persistent, batched WebSocket subscriptions to Deriv
// for continuous live tick prices — separate from the on-demand candle fetches
// used by the scanner. If the stream drops, it reconnects with backoff; if it
// stays down, callers simply fall back to the last known candle close (handled
// by LiveTickStore.Get's freshness check), so a stream outage never blocks signals.
type DerivStreamer struct {
	wsURL   string
	appID   string
	store   *LiveTickStore
	batches [][]types.Market
	stopCh  chan struct{}
	connMu  sync.Mutex
}

const batchSize = 3

func NewDerivStreamer(wsURL, appID string, markets []types.Market, store *LiveTickStore) *DerivStreamer {
	batches := [][]types.Market{}
	for i := 0; i < len(markets); i += batchSize {
		end := i + batchSize
		if end > len(markets) {
			end = len(markets)
		}
		batches = append(batches, markets[i:end])
	}
	return &DerivStreamer{
		wsURL:   wsURL,
		appID:   appID,
		store:   store,
		batches: batches,
		stopCh:  make(chan struct{}),
	}
}

// Start launches one persistent connection goroutine per batch, staggered to
// avoid opening many WebSocket connections to Deriv all at once.
func (d *DerivStreamer) Start() {
	log.Printf("streamer: starting %d tick-stream batches", len(d.batches))
	for i, batch := range d.batches {
		go d.runBatch(i, batch)
		time.Sleep(1500 * time.Millisecond)
	}
}

func (d *DerivStreamer) Stop() {
	close(d.stopCh)
}

func (d *DerivStreamer) runBatch(idx int, batch []types.Market) {
	backoff := 5 * time.Second
	for {
		select {
		case <-d.stopCh:
			return
		default:
		}

		if err := d.connectAndStream(idx, batch); err != nil {
			log.Printf("streamer: batch %d error: %v — retrying in %v", idx, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		// connection ended cleanly (shouldn't normally happen) — reset backoff and retry
		backoff = 5 * time.Second
		time.Sleep(2 * time.Second)
	}
}

func (d *DerivStreamer) connectAndStream(idx int, batch []types.Market) error {
	url := d.wsURL
	if d.appID != "" { // legacy API only; new public endpoint needs no app_id
		url = fmt.Sprintf("%s?app_id=%s", d.wsURL, d.appID)
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	// Subscribe to each symbol in this batch, staggered slightly.
	for _, m := range batch {
		msg := map[string]interface{}{"ticks": m.Deriv, "subscribe": 1}
		if err := conn.WriteJSON(msg); err != nil {
			return fmt.Errorf("subscribe %s failed: %w", m.Symbol, err)
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Map deriv symbol -> our display symbol for fast lookup on incoming ticks.
	symbolMap := make(map[string]string, len(batch))
	for _, m := range batch {
		symbolMap[m.Deriv] = m.Symbol
	}

	log.Printf("streamer: batch %d connected, streaming %d symbols", idx, len(batch))

	// Keepalive ping loop
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-d.stopCh:
				return
			case <-ticker.C:
				if err := conn.WriteJSON(map[string]interface{}{"ping": 1}); err != nil {
					return
				}
			}
		}
	}()
	defer close(pingDone)

	type tickMsg struct {
		MsgType string `json:"msg_type"`
		Tick    *struct {
			Bid    float64 `json:"bid"`
			Ask    float64 `json:"ask"`
			Quote  float64 `json:"quote"`
			Symbol string  `json:"symbol"`
		} `json:"tick"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	for {
		select {
		case <-d.stopCh:
			return nil
		default:
		}

		var msg tickMsg
		if err := conn.ReadJSON(&msg); err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		if msg.Error != nil {
			log.Printf("streamer: batch %d deriv error: %s", idx, msg.Error.Message)
			continue
		}

		if msg.MsgType == "tick" && msg.Tick != nil {
			displaySymbol, ok := symbolMap[msg.Tick.Symbol]
			if !ok {
				continue
			}
			price := msg.Tick.Quote
			if price == 0 && msg.Tick.Bid > 0 && msg.Tick.Ask > 0 {
				price = (msg.Tick.Bid + msg.Tick.Ask) / 2
			}
			if price > 0 {
				d.store.Set(displaySymbol, price)
			}
		}
	}
}
