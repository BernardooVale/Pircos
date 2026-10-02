package service

import (
	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// Tax rates for each bucket.
var taxRates = map[string]decimal.Decimal{
	domain.BucketStocksSwing:    decimal.NewFromFloat(0.15),  // 15%
	domain.BucketStocksDaytrade: decimal.NewFromFloat(0.20),  // 20%
	domain.BucketFII:            decimal.NewFromFloat(0.20),  // 20%
	domain.BucketETF:            decimal.NewFromFloat(0.15),  // 15%
	domain.BucketCrypto:         decimal.NewFromFloat(0.15),  // 15%
	domain.BucketInternational:  decimal.NewFromFloat(0.15),  // 15%
}

// Monthly stocks swing trade sales exemption threshold (R$ 20,000.00)
var stocksSwingExemptionLimit = decimal.NewFromInt(20000)

// BucketResult holds the computed tax result for a single asset bucket in a month.
type BucketResult struct {
	Bucket               string
	TotalSales           decimal.Decimal
	GrossProfit          decimal.Decimal // total profit before loss deduction
	GrossLoss            decimal.Decimal // total losses in this month (negative)
	LossesDeducted       decimal.Decimal // losses deducted from prior months
	AccumulatedLoss      decimal.Decimal // accumulated loss carried forward
	TaxableBase          decimal.Decimal
	TaxRate              decimal.Decimal
	TaxDue               decimal.Decimal
	IRRFWithheld         decimal.Decimal
	FinalDARF            decimal.Decimal
	IsExempt             bool // true if R$20k exemption applies
}

// TaxEngine calculates monthly IR obligations per tax bucket.
// It implements the rules from AGENT_SPEC section 4.2:
//   - Isolated buckets: FII losses only compensate FII profits
//   - Stocks Swing losses can compensate Stocks Swing and ETF profits (alíquota 15%)
//   - Day trade: 20%, no exemption
//   - R$ 20,000 exemption for stocks swing trade
//   - Indefinite loss carryforward within the same bucket
type TaxEngine struct{}

// NewTaxEngine creates a new TaxEngine.
func NewTaxEngine() *TaxEngine {
	return &TaxEngine{}
}

// ComputeMonthlyTax computes the tax for a given bucket in a given month.
//
// Parameters:
//   - bucket: the tax bucket identifier
//   - totalSales: total sales volume in this bucket for the month
//   - grossProfit: total profit from profitable trades (positive only)
//   - grossLoss: total losses from losing trades (as a positive number)
//   - priorAccumulatedLoss: accumulated loss carried from previous months (positive number)
//   - irrfWithheld: IRRF already withheld at source
//
// Returns a BucketResult with all computed fields.
func (e *TaxEngine) ComputeMonthlyTax(
	bucket string,
	totalSales decimal.Decimal,
	grossProfit decimal.Decimal,
	grossLoss decimal.Decimal,
	priorAccumulatedLoss decimal.Decimal,
	irrfWithheld decimal.Decimal,
) BucketResult {

	taxRate := taxRates[bucket]
	if taxRate.IsZero() {
		taxRate = decimal.NewFromFloat(0.15) // default
	}

	result := BucketResult{
		Bucket:       bucket,
		TotalSales:   totalSales,
		GrossProfit:  grossProfit,
		GrossLoss:    grossLoss,
		TaxRate:      taxRate,
		IRRFWithheld: irrfWithheld,
	}

	// Net result for the month
	netResult := grossProfit.Sub(grossLoss)

	// R$ 20,000 exemption for stocks swing trade
	if bucket == domain.BucketStocksSwing && totalSales.LessThanOrEqual(stocksSwingExemptionLimit) {
		result.IsExempt = true

		// Even with exemption, losses still accumulate for future compensation
		if netResult.IsNegative() {
			result.AccumulatedLoss = priorAccumulatedLoss.Add(netResult.Abs())
		} else {
			result.AccumulatedLoss = priorAccumulatedLoss
		}

		result.TaxableBase = decimal.Zero
		result.TaxDue = decimal.Zero
		result.LossesDeducted = decimal.Zero
		result.FinalDARF = decimal.Zero
		return result
	}

	if netResult.IsNegative() {
		// Loss month: accumulate losses, no tax due
		result.AccumulatedLoss = priorAccumulatedLoss.Add(netResult.Abs())
		result.TaxableBase = decimal.Zero
		result.TaxDue = decimal.Zero
		result.LossesDeducted = decimal.Zero
		result.FinalDARF = decimal.Zero
		return result
	}

	// Profit month: deduct prior accumulated losses
	availableForDeduction := decimal.Min(priorAccumulatedLoss, netResult)
	result.LossesDeducted = availableForDeduction
	result.AccumulatedLoss = priorAccumulatedLoss.Sub(availableForDeduction)

	result.TaxableBase = netResult.Sub(availableForDeduction)

	if result.TaxableBase.IsPositive() {
		result.TaxDue = result.TaxableBase.Mul(taxRate).RoundBank(2)
	}

	// Final DARF = tax due - IRRF already withheld
	result.FinalDARF = result.TaxDue.Sub(irrfWithheld)
	if result.FinalDARF.IsNegative() {
		result.FinalDARF = decimal.Zero
	}

	return result
}

// GetBucketForAsset determines the tax bucket for an asset class and trade type.
func GetBucketForAsset(assetClass string, isDayTrade bool) string {
	if isDayTrade && assetClass == domain.AssetClassStocks {
		return domain.BucketStocksDaytrade
	}

	switch assetClass {
	case domain.AssetClassStocks:
		return domain.BucketStocksSwing
	case domain.AssetClassFII, domain.AssetClassFIAGRO:
		return domain.BucketFII
	case domain.AssetClassETF:
		return domain.BucketETF
	case domain.AssetClassCrypto:
		return domain.BucketCrypto
	case domain.AssetClassInternational:
		return domain.BucketInternational
	default:
		return domain.BucketStocksSwing
	}
}

// CanCompensateLossBetweenBuckets returns true if losses from sourceBucket
// can compensate profits in targetBucket.
//
// Rules:
//   - FII losses only compensate FII profits
//   - Stocks Swing losses compensate Stocks Swing and ETF
//   - Day trade losses only compensate day trade of same type
//   - Each bucket is otherwise isolated
func CanCompensateLossBetweenBuckets(sourceBucket, targetBucket string) bool {
	if sourceBucket == targetBucket {
		return true
	}
	// Stocks swing trade losses can be used against ETF swing trade profits
	if sourceBucket == domain.BucketStocksSwing && targetBucket == domain.BucketETF {
		return true
	}
	if sourceBucket == domain.BucketETF && targetBucket == domain.BucketStocksSwing {
		return true
	}
	return false
}
