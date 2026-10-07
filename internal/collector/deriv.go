package collector

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"otc-predictor/pkg/types"
)

type DerivClient struct {
	AppID string
	WSURL string
}

func NewDerivClient(appID, wsURL string) *DerivClient {
	return &DerivClient{AppID: appID, WSURL: wsURL}
}

type candlesRequest struct {
	TicksHistory    string `json:"ticks_history"`
	AdjustStartTime int    `json:"adjust_start_time"`
	Count           int    `json:"count"`
	End             string `json:"end"`
	Granularity     int    `json:"granularity"`
	Style           string `json:"style"`
}

type candlesResponse struct {
	Candles []struct {
		Open  float64 `json:"open"`
		High  float64 `json:"high"`
		Low   float64 `json:"low"`
		Close float64 `json:"close"`
		Epoch int64   `json:"epoch"`
	} `json:"candles"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// FetchCandles connects, requests candle history, and returns parsed candles.
func (d *DerivClient) FetchCandles(symbol string, granularity, count int) ([]types.Candle, error) {
	url := d.WSURL
	if d.AppID != "" { // legacy API only; new public endpoint needs no app_id
		url = fmt.Sprintf("%s?app_id=%s", d.WSURL, d.AppID)
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(20 * time.Second))

	req := candlesRequest{
		TicksHistory:    symbol,
		AdjustStartTime: 1,
		Count:           count,
		End:             "latest",
		Granularity:     granularity,
		Style:           "candles",
	}
	if err := conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("write error: %w", err)
	}

	var resp candlesResponse
	if err := conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("deriv error: %s", resp.Error.Message)
	}

	candles := make([]types.Candle, 0, len(resp.Candles))
	for _, c := range resp.Candles {
		candles = append(candles, types.Candle{
			Open: c.Open, High: c.High, Low: c.Low, Close: c.Close,
			Time: c.Epoch * 1000,
		})
	}
	return candles, nil
}

// Ensure encoding/json import is used (kept for future tick subscription support)
var _ = json.Marshal
