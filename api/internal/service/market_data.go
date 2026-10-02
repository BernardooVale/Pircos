package service

import (
	"context"
	"log"
	"time"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
)

// DefaultQuoteFreshness is the maximum age for a cached quote to be considered fresh.
const DefaultQuoteFreshness = 1 * time.Hour

// MarketDataService orchestrates fetching, caching, and fallback for market data.
// It tries external APIs first, falls back to cached data when APIs are unavailable.
type MarketDataService struct {
	quoteService     *QuoteService
	benchmarkService *BenchmarkService
	quoteRepo        *repository.MarketQuoteRepo
	benchmarkRepo    *repository.MacroBenchmarkRepo
}

// NewMarketDataService creates a fully wired MarketDataService.
func NewMarketDataService(
	quoteRepo *repository.MarketQuoteRepo,
	benchmarkRepo *repository.MacroBenchmarkRepo,
) *MarketDataService {
	return &MarketDataService{
		quoteService:     NewQuoteService(),
		benchmarkService: NewBenchmarkService(),
		quoteRepo:        quoteRepo,
		benchmarkRepo:    benchmarkRepo,
	}
}

// GetQuote fetches a quote for a ticker with fallback to cache.
// Flow: try external API -> cache result -> on failure, try cached value.
func (m *MarketDataService) GetQuote(ctx context.Context, ticker string, assetClass string) (*domain.MarketQuote, error) {
	// Try fetching from external API
	quote, err := m.quoteService.FetchQuote(ctx, ticker, assetClass)
	if err == nil {
		// Cache the fresh quote
		if cacheErr := m.quoteRepo.Upsert(ctx, quote); cacheErr != nil {
			log.Printf("[MARKET] Failed to cache quote for %s: %v", ticker, cacheErr)
		}
		return quote, nil
	}

	log.Printf("[MARKET] External API failed for %s, trying cache: %v", ticker, err)

	// Fallback to cached value
	cached, cacheErr := m.quoteRepo.GetByTicker(ctx, ticker)
	if cacheErr != nil {
		return nil, err // return original API error, not the cache error
	}

	log.Printf("[MARKET] Using cached quote for %s (from %v)", ticker, cached.UpdatedAt)
	return cached, nil
}

// GetQuotes fetches quotes for multiple assets with fallback to cache.
func (m *MarketDataService) GetQuotes(ctx context.Context, assets []domain.Asset) map[string]*domain.MarketQuote {
	results := m.quoteService.FetchMultipleQuotes(ctx, assets)

	// Cache successfully fetched quotes
	var toCache []*domain.MarketQuote
	for _, q := range results {
		toCache = append(toCache, q)
	}
	if len(toCache) > 0 {
		if err := m.quoteRepo.UpsertMany(ctx, toCache); err != nil {
			log.Printf("[MARKET] Failed to cache %d quotes: %v", len(toCache), err)
		}
	}

	// For any missing tickers, try fallback to cache
	for _, asset := range assets {
		if _, ok := results[asset.Ticker]; !ok {
			cached, err := m.quoteRepo.GetByTicker(ctx, asset.Ticker)
			if err == nil {
				log.Printf("[MARKET] Using cached quote for %s (from %v)", asset.Ticker, cached.UpdatedAt)
				results[asset.Ticker] = cached
			}
		}
	}

	return results
}

// SyncBenchmarks fetches and persists benchmark data incrementally.
// It fetches from the last known date up to today.
func (m *MarketDataService) SyncBenchmarks(ctx context.Context) error {
	seriesNames := []string{"CDI", "SELIC", "IPCA"}
	seriesCodes := map[string]int{
		"CDI":   SGSSeriesCDI,
		"SELIC": SGSSeriesSelic,
		"IPCA":  SGSSeriesIPCA,
	}

	today := time.Now()

	for _, name := range seriesNames {
		// Find the last known date for this series
		lastDate, err := m.benchmarkRepo.GetLatestDate(ctx, name)
		if err != nil {
			log.Printf("[MARKET] Failed to get latest date for %s: %v", name, err)
			lastDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		}

		// Fetch from the day after the last known date
		startDate := lastDate.AddDate(0, 0, 1)
		if startDate.After(today) {
			log.Printf("[MARKET] Benchmark %s is up to date", name)
			continue
		}

		code := seriesCodes[name]
		data, err := m.benchmarkService.FetchSeries(ctx, code, startDate, today)
		if err != nil {
			log.Printf("[MARKET] Failed to fetch %s benchmarks: %v", name, err)
			continue
		}

		if len(data) > 0 {
			if err := m.benchmarkRepo.UpsertMany(ctx, data); err != nil {
				log.Printf("[MARKET] Failed to persist %s benchmarks: %v", name, err)
				continue
			}
			log.Printf("[MARKET] Synced %d %s data points (%v to %v)",
				len(data), name, startDate.Format("2006-01-02"), today.Format("2006-01-02"))
		}
	}

	return nil
}

// GetBenchmarks retrieves benchmark data from the local cache.
// Falls back to fetching from API if cache is empty for the range.
func (m *MarketDataService) GetBenchmarks(ctx context.Context, seriesName string, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	// Try from cache first
	data, err := m.benchmarkRepo.GetBySeriesAndRange(ctx, seriesName, startDate, endDate)
	if err == nil && len(data) > 0 {
		return data, nil
	}

	// Cache miss — try fetching from API
	code, ok := map[string]int{
		"CDI":   SGSSeriesCDI,
		"SELIC": SGSSeriesSelic,
		"IPCA":  SGSSeriesIPCA,
	}[seriesName]
	if !ok {
		return nil, nil
	}

	freshData, fetchErr := m.benchmarkService.FetchSeries(ctx, code, startDate, endDate)
	if fetchErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, fetchErr
	}

	// Persist fetched data
	if len(freshData) > 0 {
		if persistErr := m.benchmarkRepo.UpsertMany(ctx, freshData); persistErr != nil {
			log.Printf("[MARKET] Failed to persist fetched %s benchmarks: %v", seriesName, persistErr)
		}
	}

	return freshData, nil
}
