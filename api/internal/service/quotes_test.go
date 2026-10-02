package service

import (
	"testing"
)

func TestToYahooTicker(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Brazilian stock", "PETR4", "PETR4.SA"},
		{"Brazilian FII", "HGLG11", "HGLG11.SA"},
		{"Brazilian ETF", "BOVA11", "BOVA11.SA"},
		{"Brazilian stock VALE3", "VALE3", "VALE3.SA"},
		{"Already has suffix", "PETR4.SA", "PETR4.SA"},
		{"International stock", "AAPL", "AAPL"},
		{"International ticker MSFT", "MSFT", "MSFT"},
		{"Index with caret", "^BVSP", "^BVSP"},
		{"Ticker with dash", "BTC-USD", "BTC-USD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toYahooTicker(tt.input)
			if result != tt.expected {
				t.Errorf("toYahooTicker(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToBinancePair(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple BTC", "BTC", "BTCUSDT"},
		{"Simple ETH", "ETH", "ETHUSDT"},
		{"Lowercase btc", "btc", "BTCUSDT"},
		{"Already USDT pair", "BTCUSDT", "BTCUSDT"},
		{"Already BRL pair", "BTCBRL", "BTCBRL"},
		{"SOL token", "SOL", "SOLUSDT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toBinancePair(tt.input)
			if result != tt.expected {
				t.Errorf("toBinancePair(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
