package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// BCB SGS series codes for macro benchmarks.
const (
	SGSSeriesSelic = 11
	SGSSeriesCDI   = 12
	SGSSeriesIPCA  = 433
)

// SGS series name mapping.
var sgsSeriesMap = map[int]string{
	SGSSeriesSelic: "SELIC",
	SGSSeriesCDI:   "CDI",
	SGSSeriesIPCA:  "IPCA",
}

const sgsBaseURL = "https://api.bcb.gov.br/dados/serie/bcdata.sgs.%d/dados"

// BenchmarkService fetches macro benchmark data from Banco Central do Brasil.
type BenchmarkService struct {
	client *http.Client
}

// NewBenchmarkService creates a new BenchmarkService with a configured HTTP client.
func NewBenchmarkService() *BenchmarkService {
	return &BenchmarkService{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// sgsDataPoint represents a single data point from the BCB SGS API.
type sgsDataPoint struct {
	Date  string `json:"data"`
	Value string `json:"valor"`
}

// FetchSeries fetches historical data for a given SGS series code
// within the specified date range.
// The BCB SGS API returns dates in DD/MM/YYYY format.
func (b *BenchmarkService) FetchSeries(ctx context.Context, seriesCode int, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	seriesName, ok := sgsSeriesMap[seriesCode]
	if !ok {
		return nil, fmt.Errorf("unknown SGS series code: %d", seriesCode)
	}

	url := fmt.Sprintf(sgsBaseURL+"?formato=json&dataInicial=%s&dataFinal=%s",
		seriesCode,
		startDate.Format("02/01/2006"),
		endDate.Format("02/01/2006"),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating BCB request for series %d: %w", seriesCode, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching BCB series %d: %w", seriesCode, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("BCB API returned status %d for series %d: %s",
			resp.StatusCode, seriesCode, string(body))
	}

	var dataPoints []sgsDataPoint
	if err := json.NewDecoder(resp.Body).Decode(&dataPoints); err != nil {
		return nil, fmt.Errorf("decoding BCB response for series %d: %w", seriesCode, err)
	}

	benchmarks := make([]domain.MacroBenchmark, 0, len(dataPoints))
	for _, dp := range dataPoints {
		refDate, err := parseBCBDate(dp.Date)
		if err != nil {
			continue // skip malformed dates
		}

		rateValue, err := decimal.NewFromString(dp.Value)
		if err != nil {
			continue // skip malformed values
		}

		benchmarks = append(benchmarks, domain.MacroBenchmark{
			SeriesName:    seriesName,
			ReferenceDate: refDate,
			RateValue:     rateValue,
		})
	}

	return benchmarks, nil
}

// FetchCDI fetches CDI rate history for the given date range.
func (b *BenchmarkService) FetchCDI(ctx context.Context, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	return b.FetchSeries(ctx, SGSSeriesCDI, startDate, endDate)
}

// FetchSelic fetches Selic rate history for the given date range.
func (b *BenchmarkService) FetchSelic(ctx context.Context, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	return b.FetchSeries(ctx, SGSSeriesSelic, startDate, endDate)
}

// FetchIPCA fetches IPCA rate history for the given date range.
func (b *BenchmarkService) FetchIPCA(ctx context.Context, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	return b.FetchSeries(ctx, SGSSeriesIPCA, startDate, endDate)
}

// FetchAllBenchmarks fetches CDI, Selic, and IPCA series for the given date range.
// Returns all data points in a single slice, sorted by series and date.
func (b *BenchmarkService) FetchAllBenchmarks(ctx context.Context, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	seriesCodes := []int{SGSSeriesCDI, SGSSeriesSelic, SGSSeriesIPCA}

	type result struct {
		data []domain.MacroBenchmark
		err  error
	}

	ch := make(chan result, len(seriesCodes))

	for _, code := range seriesCodes {
		go func(c int) {
			data, err := b.FetchSeries(ctx, c, startDate, endDate)
			ch <- result{data: data, err: err}
		}(code)
	}

	var allBenchmarks []domain.MacroBenchmark
	var errors []error

	for range seriesCodes {
		r := <-ch
		if r.err != nil {
			errors = append(errors, r.err)
			continue
		}
		allBenchmarks = append(allBenchmarks, r.data...)
	}

	// Return data even if some series failed, but report errors
	if len(errors) > 0 && len(allBenchmarks) == 0 {
		return nil, fmt.Errorf("all benchmark fetches failed: %v", errors)
	}

	return allBenchmarks, nil
}

// parseBCBDate parses a date in DD/MM/YYYY format as used by BCB SGS API.
func parseBCBDate(dateStr string) (time.Time, error) {
	return time.Parse("02/01/2006", dateStr)
}
