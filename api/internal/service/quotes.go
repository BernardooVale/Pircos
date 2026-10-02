package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// QuoteProvider defines the interface for fetching market quotes.
type QuoteProvider interface {
	FetchQuote(ctx context.Context, ticker string) (*domain.MarketQuote, error)
}

// QuoteService orchestrates fetching quotes from multiple providers
// based on asset class.
type QuoteService struct {
	yahoo   *YahooFinanceClient
	binance *BinanceClient
	client  *http.Client
}

// NewQuoteService creates a new QuoteService with configured HTTP clients.
func NewQuoteService() *QuoteService {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	return &QuoteService{
		yahoo:   &YahooFinanceClient{client: client},
		binance: &BinanceClient{client: client},
		client:  client,
	}
}

// FetchQuote fetches a quote for the given ticker, routing to the appropriate
// provider based on the asset class.
func (s *QuoteService) FetchQuote(ctx context.Context, ticker string, assetClass string) (*domain.MarketQuote, error) {
	switch assetClass {
	case domain.AssetClassCrypto:
		return s.binance.FetchQuote(ctx, ticker)
	default:
		return s.yahoo.FetchQuote(ctx, ticker)
	}
}

// FetchMultipleQuotes fetches quotes for multiple tickers concurrently.
// Returns a map of ticker -> quote. Errors for individual tickers are logged
// but do not prevent other tickers from being fetched.
func (s *QuoteService) FetchMultipleQuotes(ctx context.Context, assets []domain.Asset) map[string]*domain.MarketQuote {
	results := make(map[string]*domain.MarketQuote)

	type result struct {
		ticker string
		quote  *domain.MarketQuote
		err    error
	}

	ch := make(chan result, len(assets))

	for _, asset := range assets {
		go func(a domain.Asset) {
			quote, err := s.FetchQuote(ctx, a.Ticker, a.AssetClass)
			ch <- result{ticker: a.Ticker, quote: quote, err: err}
		}(asset)
	}

	for range assets {
		r := <-ch
		if r.err != nil {
			log.Printf("[QUOTES] Failed to fetch quote for %s: %v", r.ticker, r.err)
			continue
		}
		results[r.ticker] = r.quote
	}

	return results
}

// ============================================================
// Yahoo Finance Chart API Client
// ============================================================

const yahooBaseURL = "https://query1.finance.yahoo.com/v8/finance/chart"

// YahooFinanceClient fetches quotes from Yahoo Finance Chart API.
type YahooFinanceClient struct {
	client *http.Client
}

// yahooChartResponse represents the Yahoo Finance v8 chart API response.
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				Symbol             string  `json:"symbol"`
			} `json:"meta"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// FetchQuote fetches a single stock/FII/ETF quote from Yahoo Finance.
// Brazilian tickers automatically get the .SA suffix appended.
func (y *YahooFinanceClient) FetchQuote(ctx context.Context, ticker string) (*domain.MarketQuote, error) {
	yahooTicker := toYahooTicker(ticker)
	url := fmt.Sprintf("%s/%s?interval=1d&range=1d", yahooBaseURL, yahooTicker)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating yahoo request: %w", err)
	}
	req.Header.Set("User-Agent", "Pircos/1.0")

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching yahoo quote for %s: %w", ticker, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yahoo API returned status %d for %s: %s", resp.StatusCode, ticker, string(body))
	}

	var chartResp yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&chartResp); err != nil {
		return nil, fmt.Errorf("decoding yahoo response for %s: %w", ticker, err)
	}

	if chartResp.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo API error for %s: %s - %s",
			ticker, chartResp.Chart.Error.Code, chartResp.Chart.Error.Description)
	}

	if len(chartResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no results from yahoo for %s", ticker)
	}

	meta := chartResp.Chart.Result[0].Meta
	price := decimal.NewFromFloat(meta.RegularMarketPrice)

	currency := meta.Currency
	if currency == "" {
		currency = "BRL"
	}

	return &domain.MarketQuote{
		Ticker:    ticker,
		Price:     price,
		Currency:  currency,
		UpdatedAt: time.Now(),
	}, nil
}

// toYahooTicker converts a Brazilian ticker to Yahoo Finance format.
// Brazilian stocks/FIIs/ETFs need the .SA suffix.
// International tickers and those already containing a dot are returned as-is.
func toYahooTicker(ticker string) string {
	// Already has a suffix (e.g., AAPL, ^BVSP, BTC-USD)
	if strings.Contains(ticker, ".") || strings.Contains(ticker, "^") || strings.Contains(ticker, "-") {
		return ticker
	}

	// Brazilian tickers typically end with a digit (PETR4, VALE3, HGLG11, BOVA11)
	if len(ticker) >= 4 {
		lastChar := ticker[len(ticker)-1]
		if lastChar >= '0' && lastChar <= '9' {
			return ticker + ".SA"
		}
	}

	// Default: return as-is (international tickers like AAPL, MSFT)
	return ticker
}

// ============================================================
// Binance Public API Client
// ============================================================

const binanceBaseURL = "https://api.binance.com/api/v3/ticker/price"

// BinanceClient fetches crypto quotes from Binance public API.
type BinanceClient struct {
	client *http.Client
}

// binancePriceResponse represents the Binance ticker price response.
type binancePriceResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// FetchQuote fetches a crypto price from Binance.
// Ticker should be in Binance format (e.g., "BTC" will be queried as "BTCUSDT").
func (b *BinanceClient) FetchQuote(ctx context.Context, ticker string) (*domain.MarketQuote, error) {
	binancePair := toBinancePair(ticker)
	url := fmt.Sprintf("%s?symbol=%s", binanceBaseURL, binancePair)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating binance request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching binance quote for %s: %w", ticker, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance API returned status %d for %s: %s", resp.StatusCode, ticker, string(body))
	}

	var priceResp binancePriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return nil, fmt.Errorf("decoding binance response for %s: %w", ticker, err)
	}

	price, err := decimal.NewFromString(priceResp.Price)
	if err != nil {
		return nil, fmt.Errorf("parsing binance price for %s: %w", ticker, err)
	}

	return &domain.MarketQuote{
		Ticker:    ticker,
		Price:     price,
		Currency:  "USD",
		UpdatedAt: time.Now(),
	}, nil
}

// toBinancePair converts a simple crypto ticker to a Binance trading pair.
// e.g., "BTC" -> "BTCUSDT", "ETH" -> "ETHUSDT"
// If already a pair (contains "USDT"), returns as-is.
func toBinancePair(ticker string) string {
	upper := strings.ToUpper(ticker)
	if strings.HasSuffix(upper, "USDT") || strings.HasSuffix(upper, "BRL") {
		return upper
	}
	return upper + "USDT"
}
