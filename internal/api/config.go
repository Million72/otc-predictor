package api

import (
	"os"

	"gopkg.in/yaml.v3"

	"otc-predictor/pkg/types"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Deriv struct {
		AppID string `yaml:"app_id"`
		WSURL string `yaml:"ws_url"`
	} `yaml:"deriv"`

	Markets struct {
		Forex     []types.Market `yaml:"forex"`
		Synthetic []types.Market `yaml:"synthetic"`
	} `yaml:"markets"`

	Timeframes map[string]types.TimeframeConfig `yaml:"timeframes"`

	ScanIntervalSeconds int `yaml:"scan_interval_seconds"`
	CandleCount         int `yaml:"candle_count"`
	HTFCandleCount      int `yaml:"htf_candle_count"`

	SignalRules struct {
		MinScore      float64 `yaml:"min_score"`
		MinMargin     float64 `yaml:"min_margin"`
		RSIOverbought float64 `yaml:"rsi_overbought"`
		RSIOversold   float64 `yaml:"rsi_oversold"`
	} `yaml:"signal_rules"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for i := range cfg.Markets.Forex {
		cfg.Markets.Forex[i].Type = "forex"
	}
	for i := range cfg.Markets.Synthetic {
		cfg.Markets.Synthetic[i].Type = "synthetic"
	}

	return &cfg, nil
}

func (c *Config) AllMarkets() []types.Market {
	all := make([]types.Market, 0, len(c.Markets.Forex)+len(c.Markets.Synthetic))
	all = append(all, c.Markets.Forex...)
	all = append(all, c.Markets.Synthetic...)
	return all
}
