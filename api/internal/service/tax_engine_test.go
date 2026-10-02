package service

import (
	"testing"

	"github.com/pircos/api/internal/domain"
)

func TestTaxEngine_StocksSwing_Profit_NoExemption(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("25000"),  // totalSales > 20k
		d("5000"),   // grossProfit
		d("0"),      // grossLoss
		d("0"),      // priorAccumulatedLoss
		d("0"),      // irrfWithheld
	)

	// 15% on 5000 = 750
	if !result.TaxDue.Equal(d("750")) {
		t.Errorf("TaxDue = %s, want 750", result.TaxDue)
	}
	if result.IsExempt {
		t.Error("expected not exempt")
	}
}

func TestTaxEngine_StocksSwing_Profit_WithExemption(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("18000"),  // totalSales <= 20k → EXEMPT
		d("3000"),   // grossProfit
		d("0"),      // grossLoss
		d("0"),      // priorAccumulatedLoss
		d("0"),      // irrfWithheld
	)

	if !result.TaxDue.IsZero() {
		t.Errorf("TaxDue = %s, want 0 (exempt)", result.TaxDue)
	}
	if !result.IsExempt {
		t.Error("expected exempt")
	}
}

func TestTaxEngine_StocksSwing_ExemptionEdge_Exactly20k(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("20000"),  // exactly 20k → EXEMPT
		d("2000"),
		d("0"),
		d("0"),
		d("0"),
	)

	if !result.IsExempt {
		t.Error("expected exempt at exactly R$20,000")
	}
}

func TestTaxEngine_StocksSwing_ExemptionEdge_Over20k(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("20000.01"), // just over → NOT exempt
		d("2000"),
		d("0"),
		d("0"),
		d("0"),
	)

	if result.IsExempt {
		t.Error("expected NOT exempt at R$20,000.01")
	}
	if !result.TaxDue.Equal(d("300")) {
		t.Errorf("TaxDue = %s, want 300", result.TaxDue)
	}
}

func TestTaxEngine_LossCarryforward(t *testing.T) {
	engine := NewTaxEngine()

	// Month 1: Loss
	r1 := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("25000"),
		d("0"),      // no profit
		d("3000"),   // 3000 loss
		d("0"),      // no prior loss
		d("0"),
	)

	if !r1.AccumulatedLoss.Equal(d("3000")) {
		t.Errorf("Month1 AccumulatedLoss = %s, want 3000", r1.AccumulatedLoss)
	}

	// Month 2: Profit, deducting prior loss
	r2 := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("25000"),
		d("5000"),           // profit
		d("0"),              // no loss
		r1.AccumulatedLoss,  // 3000 from month 1
		d("0"),
	)

	// Taxable = 5000 - 3000 = 2000, Tax = 2000 * 0.15 = 300
	if !r2.TaxableBase.Equal(d("2000")) {
		t.Errorf("Month2 TaxableBase = %s, want 2000", r2.TaxableBase)
	}
	if !r2.TaxDue.Equal(d("300")) {
		t.Errorf("Month2 TaxDue = %s, want 300", r2.TaxDue)
	}
	if !r2.AccumulatedLoss.IsZero() {
		t.Errorf("Month2 AccumulatedLoss = %s, want 0", r2.AccumulatedLoss)
	}
}

func TestTaxEngine_PartialLossDeduction(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("25000"),
		d("2000"),   // profit
		d("0"),      // no new loss
		d("5000"),   // prior accumulated loss > profit
		d("0"),
	)

	// Can only deduct 2000 of the 5000 loss
	if !result.LossesDeducted.Equal(d("2000")) {
		t.Errorf("LossesDeducted = %s, want 2000", result.LossesDeducted)
	}
	if !result.AccumulatedLoss.Equal(d("3000")) {
		t.Errorf("AccumulatedLoss = %s, want 3000 (remaining)", result.AccumulatedLoss)
	}
	if !result.TaxDue.IsZero() {
		t.Errorf("TaxDue = %s, want 0", result.TaxDue)
	}
}

func TestTaxEngine_FII_NoExemption(t *testing.T) {
	engine := NewTaxEngine()

	// FIIs have no R$20k exemption
	result := engine.ComputeMonthlyTax(
		domain.BucketFII,
		d("5000"),   // low sales
		d("1000"),   // profit
		d("0"),
		d("0"),
		d("0"),
	)

	// 20% on 1000 = 200
	if !result.TaxDue.Equal(d("200")) {
		t.Errorf("TaxDue = %s, want 200", result.TaxDue)
	}
	if result.IsExempt {
		t.Error("FII should never be exempt")
	}
}

func TestTaxEngine_DayTrade(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksDaytrade,
		d("50000"),
		d("2000"),
		d("0"),
		d("0"),
		d("100"), // IRRF withheld
	)

	// 20% on 2000 = 400, DARF = 400 - 100 = 300
	if !result.TaxDue.Equal(d("400")) {
		t.Errorf("TaxDue = %s, want 400", result.TaxDue)
	}
	if !result.FinalDARF.Equal(d("300")) {
		t.Errorf("FinalDARF = %s, want 300", result.FinalDARF)
	}
}

func TestTaxEngine_IRRF_ExceedsTax(t *testing.T) {
	engine := NewTaxEngine()

	result := engine.ComputeMonthlyTax(
		domain.BucketStocksDaytrade,
		d("50000"),
		d("100"),
		d("0"),
		d("0"),
		d("50"), // IRRF > tax due (20% of 100 = 20)
	)

	// Tax = 20, DARF = max(0, 20-50) = 0
	if !result.TaxDue.Equal(d("20")) {
		t.Errorf("TaxDue = %s, want 20", result.TaxDue)
	}
	if !result.FinalDARF.IsZero() {
		t.Errorf("FinalDARF = %s, want 0 (IRRF exceeds tax)", result.FinalDARF)
	}
}

func TestTaxEngine_LossInExemptMonth_Accumulates(t *testing.T) {
	engine := NewTaxEngine()

	// Loss in a month with sales < R$20k (exempt) should still accumulate
	result := engine.ComputeMonthlyTax(
		domain.BucketStocksSwing,
		d("15000"),  // exempt
		d("0"),      // no profit
		d("2000"),   // loss
		d("1000"),   // prior loss
		d("0"),
	)

	// Total accumulated = 1000 + 2000 = 3000
	if !result.AccumulatedLoss.Equal(d("3000")) {
		t.Errorf("AccumulatedLoss = %s, want 3000", result.AccumulatedLoss)
	}
}

func TestGetBucketForAsset(t *testing.T) {
	tests := []struct {
		assetClass string
		isDayTrade bool
		expected   string
	}{
		{domain.AssetClassStocks, false, domain.BucketStocksSwing},
		{domain.AssetClassStocks, true, domain.BucketStocksDaytrade},
		{domain.AssetClassFII, false, domain.BucketFII},
		{domain.AssetClassFIAGRO, false, domain.BucketFII},
		{domain.AssetClassETF, false, domain.BucketETF},
		{domain.AssetClassCrypto, false, domain.BucketCrypto},
		{domain.AssetClassInternational, false, domain.BucketInternational},
	}

	for _, tt := range tests {
		t.Run(tt.assetClass, func(t *testing.T) {
			result := GetBucketForAsset(tt.assetClass, tt.isDayTrade)
			if result != tt.expected {
				t.Errorf("GetBucketForAsset(%s, %v) = %s, want %s",
					tt.assetClass, tt.isDayTrade, result, tt.expected)
			}
		})
	}
}

func TestCanCompensateLossBetweenBuckets(t *testing.T) {
	tests := []struct {
		source   string
		target   string
		expected bool
	}{
		{domain.BucketStocksSwing, domain.BucketStocksSwing, true},
		{domain.BucketStocksSwing, domain.BucketETF, true},
		{domain.BucketETF, domain.BucketStocksSwing, true},
		{domain.BucketFII, domain.BucketFII, true},
		{domain.BucketFII, domain.BucketStocksSwing, false},
		{domain.BucketStocksSwing, domain.BucketFII, false},
		{domain.BucketStocksDaytrade, domain.BucketStocksSwing, false},
		{domain.BucketCrypto, domain.BucketStocksSwing, false},
	}

	for _, tt := range tests {
		name := tt.source + " -> " + tt.target
		t.Run(name, func(t *testing.T) {
			result := CanCompensateLossBetweenBuckets(tt.source, tt.target)
			if result != tt.expected {
				t.Errorf("CanCompensate(%s, %s) = %v, want %v",
					tt.source, tt.target, result, tt.expected)
			}
		})
	}
}
