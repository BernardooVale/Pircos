package service

import (
	"context"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
)

// PortfolioPosition represents a currently held position with both cost basis and market value.
type PortfolioPosition struct {
	AssetID         string          `json:"asset_id"`
	Ticker          string          `json:"ticker"`
	Name            string          `json:"name"`
	AssetClass      string          `json:"asset_class"`
	Currency        string          `json:"currency"`
	Quantity        decimal.Decimal `json:"quantity"`
	AveragePrice    decimal.Decimal `json:"average_price"`
	TotalCost       decimal.Decimal `json:"total_cost"`
	CurrentPrice    decimal.Decimal `json:"current_price"`
	MarketValue     decimal.Decimal `json:"market_value"`
	ProfitLoss      decimal.Decimal `json:"profit_loss"`
	ProfitLossPct   decimal.Decimal `json:"profit_loss_pct"`
	PriceUpdatedAt  *time.Time      `json:"price_updated_at,omitempty"`
}

// ClassAllocation represents the asset allocation breakdown by class.
type ClassAllocation struct {
	AssetClass  string          `json:"asset_class"`
	TotalCost   decimal.Decimal `json:"total_cost"`
	MarketValue decimal.Decimal `json:"market_value"`
	WeightPct   decimal.Decimal `json:"weight_pct"`
}

// PortfolioSummary represents the complete portfolio overview.
type PortfolioSummary struct {
	TotalCost      decimal.Decimal     `json:"total_cost"`
	MarketValue    decimal.Decimal     `json:"market_value"`
	ProfitLoss     decimal.Decimal     `json:"profit_loss"`
	ProfitLossPct  decimal.Decimal     `json:"profit_loss_pct"`
	Positions      []PortfolioPosition `json:"positions"`
	Allocations    []ClassAllocation   `json:"allocations"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

// PortfolioService computes portfolio positions, current valuations, and allocations.
type PortfolioService struct {
	txRepo        *repository.TransactionRepo
	assetRepo     *repository.AssetRepo
	earningRepo   *repository.EarningRepo
	marketData    *MarketDataService
	benchmarkRepo *repository.MacroBenchmarkRepo
}

// NewPortfolioService creates a new PortfolioService.
func NewPortfolioService(
	txRepo *repository.TransactionRepo,
	assetRepo *repository.AssetRepo,
	earningRepo *repository.EarningRepo,
	marketData *MarketDataService,
	benchmarkRepo *repository.MacroBenchmarkRepo,
) *PortfolioService {
	return &PortfolioService{
		txRepo:        txRepo,
		assetRepo:     assetRepo,
		earningRepo:   earningRepo,
		marketData:    marketData,
		benchmarkRepo: benchmarkRepo,
	}
}

// GetSummary calculates current positions, market values, and asset class allocations.
func (s *PortfolioService) GetSummary(ctx context.Context, userID string) (*PortfolioSummary, error) {
	transactions, err := s.txRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	assetMap, err := s.assetRepo.GetAllMap(ctx)
	if err != nil {
		return nil, err
	}

	var earnings []domain.Earning
	if s.earningRepo != nil {
		earnings, _ = s.earningRepo.GetByUserID(ctx, userID)
	}

	// 1. Process positions using AveragePriceEngine
	_, swingTxs := ClassifyTrades(transactions)
	pmEngine := NewAveragePriceEngine()

	// Apply amortizations
	for _, earn := range earnings {
		if earn.EarningType == domain.EarningAmortization {
			pmEngine.ApplyAmortization(earn.AssetID, earn.GrossAmount)
		}
	}

	_, _ = pmEngine.ProcessTransactions(swingTxs)
	rawPositions := pmEngine.GetAllPositions()

	// 2. Fetch quotes and build enriched positions
	var positions []PortfolioPosition
	totalCost := decimal.Zero
	totalMarketValue := decimal.Zero

	classCostMap := make(map[string]decimal.Decimal)
	classMarketMap := make(map[string]decimal.Decimal)

	for _, pos := range rawPositions {
		if !pos.Quantity.IsPositive() {
			continue
		}

		asset, ok := assetMap[pos.AssetID]
		if !ok {
			asset = domain.Asset{
				ID:         pos.AssetID,
				Ticker:     pos.AssetID,
				Name:       pos.AssetID,
				AssetClass: domain.AssetClassStocks,
			}
		}

		currentPrice := pos.AveragePrice // fallback if quote unavailable
		var priceUpdatedAt *time.Time

		if s.marketData != nil {
			quote, err := s.marketData.GetQuote(ctx, asset.Ticker, asset.AssetClass)
			if err == nil && quote != nil && quote.Price.IsPositive() {
				currentPrice = quote.Price
				priceUpdatedAt = &quote.UpdatedAt
			}
		}

		marketValue := pos.Quantity.Mul(currentPrice)
		pl := marketValue.Sub(pos.TotalCost)
		var plPct decimal.Decimal
		if pos.TotalCost.IsPositive() {
			plPct = pl.Div(pos.TotalCost).Mul(decimal.NewFromInt(100)).RoundBank(2)
		}

		totalCost = totalCost.Add(pos.TotalCost)
		totalMarketValue = totalMarketValue.Add(marketValue)

		classCostMap[asset.AssetClass] = classCostMap[asset.AssetClass].Add(pos.TotalCost)
		classMarketMap[asset.AssetClass] = classMarketMap[asset.AssetClass].Add(marketValue)

		positions = append(positions, PortfolioPosition{
			AssetID:        pos.AssetID,
			Ticker:         asset.Ticker,
			Name:           asset.Name,
			AssetClass:     asset.AssetClass,
			Currency:       asset.Currency,
			Quantity:       pos.Quantity,
			AveragePrice:   pos.AveragePrice.RoundBank(4),
			TotalCost:      pos.TotalCost.RoundBank(2),
			CurrentPrice:   currentPrice.RoundBank(4),
			MarketValue:    marketValue.RoundBank(2),
			ProfitLoss:     pl.RoundBank(2),
			ProfitLossPct:  plPct,
			PriceUpdatedAt: priceUpdatedAt,
		})
	}

	// Sort positions by market value descending
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].MarketValue.GreaterThan(positions[j].MarketValue)
	})

	// 3. Compute allocations
	var allocations []ClassAllocation
	for class, mVal := range classMarketMap {
		cost := classCostMap[class]
		var weight decimal.Decimal
		if totalMarketValue.IsPositive() {
			weight = mVal.Div(totalMarketValue).Mul(decimal.NewFromInt(100)).RoundBank(2)
		}
		allocations = append(allocations, ClassAllocation{
			AssetClass:  class,
			TotalCost:   cost.RoundBank(2),
			MarketValue: mVal.RoundBank(2),
			WeightPct:   weight,
		})
	}
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].MarketValue.GreaterThan(allocations[j].MarketValue)
	})

	totalPL := totalMarketValue.Sub(totalCost)
	var totalPLPct decimal.Decimal
	if totalCost.IsPositive() {
		totalPLPct = totalPL.Div(totalCost).Mul(decimal.NewFromInt(100)).RoundBank(2)
	}

	return &PortfolioSummary{
		TotalCost:     totalCost.RoundBank(2),
		MarketValue:   totalMarketValue.RoundBank(2),
		ProfitLoss:    totalPL.RoundBank(2),
		ProfitLossPct: totalPLPct,
		Positions:     positions,
		Allocations:   allocations,
		UpdatedAt:     time.Now(),
	}, nil
}

// HistoryPoint represents portfolio valuation at a point in time.
type HistoryPoint struct {
	Date        string          `json:"date"` // YYYY-MM
	TotalCost   decimal.Decimal `json:"total_cost"`
	MarketValue decimal.Decimal `json:"market_value"`
	CDIRate     decimal.Decimal `json:"cdi_rate,omitempty"`
	SelicRate   decimal.Decimal `json:"selic_rate,omitempty"`
	IPCARate    decimal.Decimal `json:"ipca_rate,omitempty"`
}

// GetHistory returns historical timeline of portfolio valuation.
func (s *PortfolioService) GetHistory(ctx context.Context, userID string) ([]HistoryPoint, error) {
	transactions, err := s.txRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(transactions) == 0 {
		return []HistoryPoint{}, nil
	}

	// Group transactions by month
	monthlyCost := make(map[string]decimal.Decimal)
	var months []string

	runningCost := decimal.Zero
	for _, tx := range transactions {
		ym := tx.OperationDate.Format("2006-01")
		if _, exists := monthlyCost[ym]; !exists {
			months = append(months, ym)
		}

		if tx.OperationType == domain.OpBuy {
			runningCost = runningCost.Add(tx.TotalAmount)
		} else if tx.OperationType == domain.OpSell {
			runningCost = runningCost.Sub(tx.TotalAmount)
			if runningCost.IsNegative() {
				runningCost = decimal.Zero
			}
		}
		monthlyCost[ym] = runningCost
	}

	sort.Strings(months)

	// Fetch benchmark data if available
	type monthBenchmark struct {
		CDI  decimal.Decimal
		Selic decimal.Decimal
		IPCA decimal.Decimal
	}
	benchmarks := make(map[string]*monthBenchmark)

	if s.benchmarkRepo != nil && len(months) > 0 {
		firstMonth, _ := time.Parse("2006-01", months[0])
		lastMonth, _ := time.Parse("2006-01", months[len(months)-1])
		lastMonth = lastMonth.AddDate(0, 1, -1) // end of last month

		for _, series := range []string{"CDI", "SELIC", "IPCA"} {
			data, err := s.benchmarkRepo.GetBySeriesAndRange(ctx, series, firstMonth, lastMonth)
			if err != nil {
				continue
			}
			for _, d := range data {
				ym := d.ReferenceDate.Format("2006-01")
				if benchmarks[ym] == nil {
					benchmarks[ym] = &monthBenchmark{}
				}
				switch series {
				case "CDI":
					benchmarks[ym].CDI = benchmarks[ym].CDI.Add(d.RateValue)
				case "SELIC":
					benchmarks[ym].Selic = benchmarks[ym].Selic.Add(d.RateValue)
				case "IPCA":
					benchmarks[ym].IPCA = benchmarks[ym].IPCA.Add(d.RateValue)
				}
			}
		}
	}

	var history []HistoryPoint
	for _, ym := range months {
		cost := monthlyCost[ym]
		hp := HistoryPoint{
			Date:        ym,
			TotalCost:   cost.RoundBank(2),
			MarketValue: cost.RoundBank(2),
		}
		if bm, ok := benchmarks[ym]; ok {
			hp.CDIRate = bm.CDI
			hp.SelicRate = bm.Selic
			hp.IPCARate = bm.IPCA
		}
		history = append(history, hp)
	}

	return history, nil
}
