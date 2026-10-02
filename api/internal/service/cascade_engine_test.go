package service

import (
	"testing"

	"github.com/pircos/api/internal/domain"
)

func TestCalculateMonthlyBalances_CascadeLossCarry(t *testing.T) {
	userID := "user-1"
	petr4 := domain.Asset{
		ID:         "asset-petr4",
		Ticker:     "PETR4",
		Name:       "Petrobras",
		AssetClass: domain.AssetClassStocks,
	}
	assets := map[string]domain.Asset{
		petr4.ID: petr4,
	}

	// Month 1 (2024-01): Buy at 30, Sell at 20 (Loss of 1000) with sales > 20k
	// Month 2 (2024-02): Buy at 20, Sell at 30 (Profit of 2000) with sales > 20k
	txs := []domain.Transaction{
		// 2024-01-05: Buy 1000 PETR4 @ 30 = 30,000
		{
			ID: "tx-1", UserID: userID, AssetID: petr4.ID, OperationType: domain.OpBuy,
			Quantity: d("1000"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("30000.00"), OperationDate: makeDate(2024, 1, 5),
		},
		// 2024-01-20: Sell 1000 PETR4 @ 20 = 20,000 (total sales = 20,000.01 to avoid exemption in test)
		{
			ID: "tx-2", UserID: userID, AssetID: petr4.ID, OperationType: domain.OpSell,
			Quantity: d("1000"), UnitPrice: d("21.00"), Costs: d("0"),
			TotalAmount: d("21000.00"), OperationDate: makeDate(2024, 1, 20),
		},
		// Net loss: 21000 - 30000 = -9000

		// 2024-02-05: Buy 1000 PETR4 @ 20 = 20,000
		{
			ID: "tx-3", UserID: userID, AssetID: petr4.ID, OperationType: domain.OpBuy,
			Quantity: d("1000"), UnitPrice: d("20.00"), Costs: d("0"),
			TotalAmount: d("20000.00"), OperationDate: makeDate(2024, 2, 5),
		},
		// 2024-02-25: Sell 1000 PETR4 @ 35 = 35,000 (profit: 35000 - 20000 = 15000)
		{
			ID: "tx-4", UserID: userID, AssetID: petr4.ID, OperationType: domain.OpSell,
			Quantity: d("1000"), UnitPrice: d("35.00"), Costs: d("0"),
			TotalAmount: d("35000.00"), OperationDate: makeDate(2024, 2, 25),
		},
	}

	balances := CalculateMonthlyBalances(userID, txs, assets, nil, NewTaxEngine())

	if len(balances) < 2 {
		t.Fatalf("expected at least 2 monthly balances, got %d", len(balances))
	}

	// Month 1
	var m1, m2 *domain.MonthlyTaxBalance
	for i := range balances {
		if balances[i].YearMonth == "2024-01" && balances[i].AssetBucket == domain.BucketStocksSwing {
			m1 = &balances[i]
		}
		if balances[i].YearMonth == "2024-02" && balances[i].AssetBucket == domain.BucketStocksSwing {
			m2 = &balances[i]
		}
	}

	if m1 == nil {
		t.Fatal("month 1 balance not found")
	}
	if !m1.AccumulatedLossCarried.Equal(d("9000")) {
		t.Errorf("Month 1 accumulated loss carried = %s, want 9000", m1.AccumulatedLossCarried)
	}
	if !m1.TaxDue.IsZero() {
		t.Errorf("Month 1 tax due = %s, want 0", m1.TaxDue)
	}

	if m2 == nil {
		t.Fatal("month 2 balance not found")
	}
	// In month 2, profit is 15000. Deduct 9000 loss => taxable base 6000.
	// Tax due = 6000 * 0.15 = 900.
	if !m2.LossesDeducted.Equal(d("9000")) {
		t.Errorf("Month 2 losses deducted = %s, want 9000", m2.LossesDeducted)
	}
	if !m2.TaxableBase.Equal(d("6000")) {
		t.Errorf("Month 2 taxable base = %s, want 6000", m2.TaxableBase)
	}
	if !m2.TaxDue.Equal(d("900")) {
		t.Errorf("Month 2 tax due = %s, want 900", m2.TaxDue)
	}
	if !m2.AccumulatedLossCarried.IsZero() {
		t.Errorf("Month 2 accumulated loss carried = %s, want 0", m2.AccumulatedLossCarried)
	}
}

func TestCalculateMonthlyBalances_BucketIsolation(t *testing.T) {
	userID := "user-1"
	petr4 := domain.Asset{
		ID:         "asset-petr4",
		Ticker:     "PETR4",
		Name:       "Petrobras",
		AssetClass: domain.AssetClassStocks,
	}
	hglg11 := domain.Asset{
		ID:         "asset-hglg11",
		Ticker:     "HGLG11",
		Name:       "CSHG Logística",
		AssetClass: domain.AssetClassFII,
	}
	assets := map[string]domain.Asset{
		petr4.ID:  petr4,
		hglg11.ID: hglg11,
	}

	// Month 1: Stock loss 5000 (sales > 20k), FII profit 2000
	// Stock loss must NOT compensate FII profit!
	txs := []domain.Transaction{
		// PETR4 buy and sell
		{
			ID: "tx-1", UserID: userID, AssetID: petr4.ID, OperationType: domain.OpBuy,
			Quantity: d("1000"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("30000.00"), OperationDate: makeDate(2024, 1, 5),
		},
		{
			ID: "tx-2", UserID: userID, AssetID: petr4.ID, OperationType: domain.OpSell,
			Quantity: d("1000"), UnitPrice: d("25.00"), Costs: d("0"),
			TotalAmount: d("25000.00"), OperationDate: makeDate(2024, 1, 15),
		},
		// HGLG11 buy and sell
		{
			ID: "tx-3", UserID: userID, AssetID: hglg11.ID, OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("100.00"), Costs: d("0"),
			TotalAmount: d("10000.00"), OperationDate: makeDate(2024, 1, 5),
		},
		{
			ID: "tx-4", UserID: userID, AssetID: hglg11.ID, OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("120.00"), Costs: d("0"),
			TotalAmount: d("12000.00"), OperationDate: makeDate(2024, 1, 20),
		},
	}

	balances := CalculateMonthlyBalances(userID, txs, assets, nil, NewTaxEngine())

	var stocksBalance, fiiBalance *domain.MonthlyTaxBalance
	for i := range balances {
		if balances[i].AssetBucket == domain.BucketStocksSwing {
			stocksBalance = &balances[i]
		}
		if balances[i].AssetBucket == domain.BucketFII {
			fiiBalance = &balances[i]
		}
	}

	if stocksBalance == nil || fiiBalance == nil {
		t.Fatal("expected both stocks and FII balances")
	}

	// Stocks has 5000 loss carried forward
	if !stocksBalance.AccumulatedLossCarried.Equal(d("5000")) {
		t.Errorf("Stocks accumulated loss = %s, want 5000", stocksBalance.AccumulatedLossCarried)
	}

	// FII has 2000 profit taxed at 20% = 400 (not compensated by stock loss)
	if !fiiBalance.TaxDue.Equal(d("400")) {
		t.Errorf("FII tax due = %s, want 400", fiiBalance.TaxDue)
	}
	if !fiiBalance.LossesDeducted.IsZero() {
		t.Errorf("FII losses deducted = %s, want 0", fiiBalance.LossesDeducted)
	}
}
