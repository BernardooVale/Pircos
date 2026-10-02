package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
)

// TransactionFetcher fetches transactions for a user.
type TransactionFetcher interface {
	GetByUserID(ctx context.Context, userID string) ([]domain.Transaction, error)
}

// AssetFetcher fetches assets by IDs.
type AssetFetcher interface {
	GetAllMap(ctx context.Context) (map[string]domain.Asset, error)
}

// EarningFetcher fetches earnings for a user.
type EarningFetcher interface {
	GetByUserID(ctx context.Context, userID string) ([]domain.Earning, error)
}

// CascadeEngine handles chronological recalculation of portfolio positions,
// trades, and monthly tax balances across all historical months.
type CascadeEngine struct {
	taxEngine      *TaxEngine
	balanceRepo    *repository.MonthlyTaxBalanceRepo
	txFetcher      TransactionFetcher
	assetFetcher   AssetFetcher
	earningFetcher EarningFetcher
}

// NewCascadeEngine creates a new CascadeEngine.
func NewCascadeEngine(
	taxEngine *TaxEngine,
	balanceRepo *repository.MonthlyTaxBalanceRepo,
	txFetcher TransactionFetcher,
	assetFetcher AssetFetcher,
	earningFetcher EarningFetcher,
) *CascadeEngine {
	return &CascadeEngine{
		taxEngine:      taxEngine,
		balanceRepo:    balanceRepo,
		txFetcher:      txFetcher,
		assetFetcher:   assetFetcher,
		earningFetcher: earningFetcher,
	}
}

// Recalculate loads all transactions, assets and earnings for a user,
// executes chronological cascade calculation, and saves snapshots to the database.
func (c *CascadeEngine) Recalculate(ctx context.Context, userID string) error {
	transactions, err := c.txFetcher.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("fetching transactions: %w", err)
	}

	assetMap, err := c.assetFetcher.GetAllMap(ctx)
	if err != nil {
		return fmt.Errorf("fetching assets: %w", err)
	}

	var earnings []domain.Earning
	if c.earningFetcher != nil {
		earnings, err = c.earningFetcher.GetByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("fetching earnings: %w", err)
		}
	}

	balances := CalculateMonthlyBalances(userID, transactions, assetMap, earnings, c.taxEngine)

	if len(balances) > 0 {
		firstMonth := balances[0].YearMonth
		if err := c.balanceRepo.DeleteFromMonth(ctx, userID, firstMonth); err != nil {
			return fmt.Errorf("cleaning stale balances from %s: %w", firstMonth, err)
		}
		if err := c.balanceRepo.UpsertMany(ctx, balances); err != nil {
			return fmt.Errorf("saving recalculated balances: %w", err)
		}
	}

	return nil
}

// monthBucketAccumulator aggregates sales and P/L for a bucket in a given month.
type monthBucketAccumulator struct {
	TotalSales   decimal.Decimal
	GrossProfit  decimal.Decimal
	GrossLoss    decimal.Decimal // positive value
	IRRFWithheld decimal.Decimal
}

// CalculateMonthlyBalances computes monthly tax balance records chronologically.
// This is a pure computation function that is easily testable.
func CalculateMonthlyBalances(
	userID string,
	transactions []domain.Transaction,
	assets map[string]domain.Asset,
	earnings []domain.Earning,
	taxEngine *TaxEngine,
) []domain.MonthlyTaxBalance {
	if len(transactions) == 0 {
		return nil
	}

	if taxEngine == nil {
		taxEngine = NewTaxEngine()
	}

	// 1. Classify day trade pairs and swing trade operations
	dtPairs, swingTxs := ClassifyTrades(transactions)

	// 2. Process amortization earnings
	amortizationsByAsset := make(map[string][]domain.Earning)
	for _, earn := range earnings {
		if earn.EarningType == domain.EarningAmortization {
			amortizationsByAsset[earn.AssetID] = append(amortizationsByAsset[earn.AssetID], earn)
		}
	}

	// 3. Process swing trades through PM engine
	pmEngine := NewAveragePriceEngine()

	// Apply amortizations to PM engine
	for assetID, earns := range amortizationsByAsset {
		for _, e := range earns {
			pmEngine.ApplyAmortization(assetID, e.GrossAmount)
		}
	}

	saleResults, err := pmEngine.ProcessTransactions(swingTxs)
	if err != nil {
		// Log or handle gracefully
		saleResults = nil
	}

	// 4. Group all sales (Day Trade + Swing Trade) by YearMonth and Bucket
	// map[yearMonth]map[bucket]*monthBucketAccumulator
	monthlyData := make(map[string]map[string]*monthBucketAccumulator)

	getAcc := func(ym, bucket string) *monthBucketAccumulator {
		if monthlyData[ym] == nil {
			monthlyData[ym] = make(map[string]*monthBucketAccumulator)
		}
		if monthlyData[ym][bucket] == nil {
			monthlyData[ym][bucket] = &monthBucketAccumulator{
				TotalSales:   decimal.Zero,
				GrossProfit:  decimal.Zero,
				GrossLoss:    decimal.Zero,
				IRRFWithheld: decimal.Zero,
			}
		}
		return monthlyData[ym][bucket]
	}

	// Process swing sales
	for _, sale := range saleResults {
		ym := sale.OperationDate.Format("2006-01")
		asset := assets[sale.AssetID]
		bucket := GetBucketForAsset(asset.AssetClass, false)

		acc := getAcc(ym, bucket)
		acc.TotalSales = acc.TotalSales.Add(sale.SaleAmount)

		if sale.ProfitLoss.IsPositive() {
			acc.GrossProfit = acc.GrossProfit.Add(sale.ProfitLoss)
		} else if sale.ProfitLoss.IsNegative() {
			acc.GrossLoss = acc.GrossLoss.Add(sale.ProfitLoss.Abs())
		}
	}

	// Process day trade pairs
	// Group day trade pairs by date and asset
	for _, pair := range dtPairs {
		ym := pair.SellTx.OperationDate.Format("2006-01")
		asset := assets[pair.SellTx.AssetID]
		bucket := GetBucketForAsset(asset.AssetClass, true)

		acc := getAcc(ym, bucket)
		salesVal := DayTradeTotalSales([]TradePair{pair})
		pl := DayTradeProfitLoss([]TradePair{pair})

		acc.TotalSales = acc.TotalSales.Add(salesVal)
		if pl.IsPositive() {
			acc.GrossProfit = acc.GrossProfit.Add(pl)
		} else if pl.IsNegative() {
			acc.GrossLoss = acc.GrossLoss.Add(pl.Abs())
		}
	}

	// 5. Sort distinct months chronologically
	var months []string
	for ym := range monthlyData {
		months = append(months, ym)
	}
	sort.Strings(months)

	// 6. Chronological cascade computation carrying accumulated losses forward per bucket
	// Track accumulated loss carried forward for each bucket
	accumulatedLossByBucket := make(map[string]decimal.Decimal)

	allBuckets := []string{
		domain.BucketStocksSwing,
		domain.BucketStocksDaytrade,
		domain.BucketFII,
		domain.BucketETF,
		domain.BucketCrypto,
		domain.BucketInternational,
	}

	var results []domain.MonthlyTaxBalance

	for _, ym := range months {
		monthBuckets := monthlyData[ym]

		// First pass: compute each bucket independently
		var monthResults []BucketResult
		for _, bucket := range allBuckets {
			acc, hasActivity := monthBuckets[bucket]
			priorLoss := accumulatedLossByBucket[bucket]

			if !hasActivity && priorLoss.IsZero() {
				continue
			}

			totalSales := decimal.Zero
			grossProfit := decimal.Zero
			grossLoss := decimal.Zero
			irrf := decimal.Zero

			if hasActivity {
				totalSales = acc.TotalSales
				grossProfit = acc.GrossProfit
				grossLoss = acc.GrossLoss
				irrf = acc.IRRFWithheld
			}

			taxResult := taxEngine.ComputeMonthlyTax(
				bucket,
				totalSales,
				grossProfit,
				grossLoss,
				priorLoss,
				irrf,
			)

			monthResults = append(monthResults, taxResult)
		}

		// Second pass: cross-bucket loss compensation between compatible buckets
		// Per spec §4.2: Stocks Swing losses can compensate ETF profits and vice-versa
		bucketResultMap := make(map[string]*BucketResult)
		for i := range monthResults {
			bucketResultMap[monthResults[i].Bucket] = &monthResults[i]
		}

		compensationPairs := [][2]string{
			{domain.BucketStocksSwing, domain.BucketETF},
			{domain.BucketETF, domain.BucketStocksSwing},
		}

		for _, pair := range compensationPairs {
			source, target := pair[0], pair[1]
			srcResult, srcOK := bucketResultMap[source]
			tgtResult, tgtOK := bucketResultMap[target]
			if !srcOK || !tgtOK {
				continue
			}
			if !srcResult.AccumulatedLoss.IsPositive() || !tgtResult.TaxableBase.IsPositive() {
				continue
			}
			transfer := decimal.Min(srcResult.AccumulatedLoss, tgtResult.TaxableBase)
			srcResult.AccumulatedLoss = srcResult.AccumulatedLoss.Sub(transfer)
			srcResult.LossesDeducted = srcResult.LossesDeducted.Add(transfer)
			tgtResult.TaxableBase = tgtResult.TaxableBase.Sub(transfer)
			tgtResult.LossesDeducted = tgtResult.LossesDeducted.Add(transfer)
			if tgtResult.TaxableBase.IsPositive() {
				tgtResult.TaxDue = tgtResult.TaxableBase.Mul(tgtResult.TaxRate).RoundBank(2)
			} else {
				tgtResult.TaxDue = decimal.Zero
			}
			tgtResult.FinalDARF = tgtResult.TaxDue.Sub(tgtResult.IRRFWithheld)
			if tgtResult.FinalDARF.IsNegative() {
				tgtResult.FinalDARF = decimal.Zero
			}
		}

		// Convert results to domain objects and update accumulated losses
		for _, taxResult := range monthResults {
			accumulatedLossByBucket[taxResult.Bucket] = taxResult.AccumulatedLoss

			results = append(results, domain.MonthlyTaxBalance{
				UserID:                 userID,
				YearMonth:              ym,
				AssetBucket:            taxResult.Bucket,
				TotalSales:             taxResult.TotalSales,
				GrossProfit:            taxResult.GrossProfit,
				LossesDeducted:         taxResult.LossesDeducted,
				AccumulatedLossCarried: taxResult.AccumulatedLoss,
				TaxableBase:            taxResult.TaxableBase,
				TaxRate:                taxResult.TaxRate,
				TaxDue:                 taxResult.TaxDue,
				IRRFWithheld:           taxResult.IRRFWithheld,
				FinalDARF:              taxResult.FinalDARF,
				UpdatedAt:              time.Now(),
			})
		}
	}

	return results
}
