package handler

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/service"
)

// BenchmarkHandler handles macro benchmark data requests.
type BenchmarkHandler struct {
	marketData *service.MarketDataService
}

// NewBenchmarkHandler creates a new BenchmarkHandler.
func NewBenchmarkHandler(marketData *service.MarketDataService) *BenchmarkHandler {
	return &BenchmarkHandler{marketData: marketData}
}

// List handles GET /api/v1/benchmarks
func (h *BenchmarkHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	endDate := time.Now()
	startDate := endDate.AddDate(-1, 0, 0) // default 1 year range

	if s := q.Get("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}
	if e := q.Get("end_date"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			endDate = t
		}
	}

	if h.marketData == nil {
		writeJSON(w, http.StatusOK, []domain.MacroBenchmark{})
		return
	}

	seriesName := q.Get("series")
	var allData []domain.MacroBenchmark

	if seriesName != "" {
		data, err := h.marketData.GetBenchmarks(ctx, seriesName, startDate, endDate)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed fetching benchmarks: "+err.Error())
			return
		}
		allData = data
	} else {
		for _, name := range []string{"CDI", "SELIC", "IPCA"} {
			data, _ := h.marketData.GetBenchmarks(ctx, name, startDate, endDate)
			allData = append(allData, data...)
		}
	}

	if allData == nil {
		allData = []domain.MacroBenchmark{}
	}

	// Normalize to base-100 cumulative index if requested
	if q.Get("normalized") == "true" {
		allData = normalizeBenchmarks(allData)
	}

	writeJSON(w, http.StatusOK, allData)
}

// normalizeBenchmarks converts raw rate data points into a cumulative base-100 index.
// For CDI/SELIC (daily rates as annual %): converts to daily factor and compounds.
// For IPCA (monthly %): compounds directly.
func normalizeBenchmarks(data []domain.MacroBenchmark) []domain.MacroBenchmark {
	// Group by series
	type seriesData struct {
		points []domain.MacroBenchmark
	}
	byName := make(map[string]*seriesData)
	for _, d := range data {
		if byName[d.SeriesName] == nil {
			byName[d.SeriesName] = &seriesData{}
		}
		byName[d.SeriesName].points = append(byName[d.SeriesName].points, d)
	}

	var result []domain.MacroBenchmark
	hundred := decimal.NewFromInt(100)
	one := decimal.NewFromInt(1)

	for name, sd := range byName {
		index := hundred
		for _, p := range sd.points {
			rate := p.RateValue
			var factor decimal.Decimal
			if name == "IPCA" {
				// IPCA is monthly percentage
				factor = one.Add(rate.Div(hundred))
			} else {
				// CDI/SELIC: annual rate stored as daily factor already in decimal form
				// The BCB SGS returns the daily rate as a percentage (e.g., 0.0534 means 0.0534%)
				factor = one.Add(rate.Div(hundred))
			}
			index = index.Mul(factor).RoundBank(6)
			result = append(result, domain.MacroBenchmark{
				SeriesName:    p.SeriesName,
				ReferenceDate: p.ReferenceDate,
				RateValue:     index,
			})
		}
	}

	return result
}
