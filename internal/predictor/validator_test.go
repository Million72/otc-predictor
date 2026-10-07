package predictor

import (
	"otc-predictor/internal/strategy"
	"otc-predictor/pkg/types"
	"testing"
)

func TestValidateSignalUsesConfiguredRules(t *testing.T) {
	result := strategy.EngineResult{
		BullScore: 12, BearScore: 0, MaxScore: 41,
		RSI: 55, HTF1Bias: "BULL", HTF2Bias: "BULL",
		Trend:       "BULLISH",
		EntryModels: []strategy.EntryModelMatch{{Side: "bull", Model: 1}},
	}
	low := types.SignalRules{MinScore: 10, MinMargin: 4, RSIOverbought: 70, RSIOversold: 30, MinConfidencePct: 20}
	if !ValidateSignal(result, low).Valid {
		t.Fatalf("signal should pass with a 20%% configured confidence floor")
	}
	high := low
	high.MinConfidencePct = 40
	if ValidateSignal(result, high).Valid {
		t.Fatalf("signal should fail with a 40%% configured confidence floor")
	}
}
