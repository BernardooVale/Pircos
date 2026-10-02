package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

func d(val string) decimal.Decimal {
	v, _ := decimal.NewFromString(val)
	return v
}

func makeDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 10, 0, 0, 0, time.UTC)
}

func TestAveragePriceEngine_SingleBuy(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("25.00"), Costs: d("10.00"),
			TotalAmount: d("2510.00"), OperationDate: makeDate(2024, 1, 15),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := engine.GetPosition("asset-1")
	// PM = (100 * 25.00 + 10.00) / 100 = 2510 / 100 = 25.10
	expectedPM := d("25.10")
	if !pos.AveragePrice.Equal(expectedPM) {
		t.Errorf("PM = %s, want %s", pos.AveragePrice, expectedPM)
	}
	if !pos.Quantity.Equal(d("100")) {
		t.Errorf("Qty = %s, want 100", pos.Quantity)
	}
}

func TestAveragePriceEngine_MultipleBuys(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("20.00"), Costs: d("10.00"),
			TotalAmount: d("2010.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("50"), UnitPrice: d("30.00"), Costs: d("5.00"),
			TotalAmount: d("1505.00"), OperationDate: makeDate(2024, 2, 10),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := engine.GetPosition("asset-1")
	// First buy: PM = (100*20 + 10) / 100 = 2010/100 = 20.10
	// Second buy: PM = (100*20.10 + 50*30 + 5) / 150 = (2010 + 1500 + 5) / 150 = 3515/150 = 23.4333...
	expectedPM := d("3515").Div(d("150"))
	if !pos.AveragePrice.Equal(expectedPM) {
		t.Errorf("PM = %s, want %s", pos.AveragePrice, expectedPM)
	}
	if !pos.Quantity.Equal(d("150")) {
		t.Errorf("Qty = %s, want 150", pos.Quantity)
	}
}

func TestAveragePriceEngine_BuyAndSell_Profit(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("20.00"), Costs: d("0"),
			TotalAmount: d("2000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "asset-1", OperationType: domain.OpSell,
			Quantity: d("50"), UnitPrice: d("30.00"), Costs: d("5.00"),
			TotalAmount: d("1500.00"), OperationDate: makeDate(2024, 3, 10),
		},
	}

	results, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 sale result, got %d", len(results))
	}

	r := results[0]
	// PM = 20.00 (no costs on buy)
	// Net sale = 1500 - 5 = 1495
	// Cost basis = 50 * 20 = 1000
	// P/L = 1495 - 1000 = 495
	if !r.ProfitLoss.Equal(d("495")) {
		t.Errorf("ProfitLoss = %s, want 495", r.ProfitLoss)
	}

	pos := engine.GetPosition("asset-1")
	// PM unchanged after sell
	if !pos.AveragePrice.Equal(d("20")) {
		t.Errorf("PM after sell = %s, want 20", pos.AveragePrice)
	}
	if !pos.Quantity.Equal(d("50")) {
		t.Errorf("Qty after sell = %s, want 50", pos.Quantity)
	}
}

func TestAveragePriceEngine_BuyAndSell_Loss(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("3000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "asset-1", OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("20.00"), Costs: d("10.00"),
			TotalAmount: d("2000.00"), OperationDate: makeDate(2024, 3, 10),
		},
	}

	results, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := results[0]
	// PM = 30.00
	// Net sale = 2000 - 10 = 1990
	// Cost basis = 100 * 30 = 3000
	// P/L = 1990 - 3000 = -1010
	if !r.ProfitLoss.Equal(d("-1010")) {
		t.Errorf("ProfitLoss = %s, want -1010", r.ProfitLoss)
	}

	pos := engine.GetPosition("asset-1")
	// Fully closed position
	if !pos.Quantity.IsZero() {
		t.Errorf("Qty after full sell = %s, want 0", pos.Quantity)
	}
	if !pos.AveragePrice.IsZero() {
		t.Errorf("PM after full sell = %s, want 0", pos.AveragePrice)
	}
}

func TestAveragePriceEngine_Split(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("40.00"), Costs: d("0"),
			TotalAmount: d("4000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "asset-1", OperationType: domain.OpSplit,
			Quantity: d("0"), UnitPrice: d("2"), Costs: d("0"),
			TotalAmount: d("0"), OperationDate: makeDate(2024, 6, 1),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := engine.GetPosition("asset-1")
	// After 1:2 split: Qty = 200, PM = 20, TotalCost = 4000 (unchanged)
	if !pos.Quantity.Equal(d("200")) {
		t.Errorf("Qty after split = %s, want 200", pos.Quantity)
	}
	if !pos.AveragePrice.Equal(d("20")) {
		t.Errorf("PM after split = %s, want 20", pos.AveragePrice)
	}
	if !pos.TotalCost.Equal(d("4000")) {
		t.Errorf("TotalCost after split = %s, want 4000", pos.TotalCost)
	}
}

func TestAveragePriceEngine_Grouping(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("200"), UnitPrice: d("10.00"), Costs: d("0"),
			TotalAmount: d("2000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			// 2:1 grouping (ratio = 0.5, qty halves)
			ID: "2", AssetID: "asset-1", OperationType: domain.OpGrouping,
			Quantity: d("0"), UnitPrice: d("0.5"), Costs: d("0"),
			TotalAmount: d("0"), OperationDate: makeDate(2024, 6, 1),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := engine.GetPosition("asset-1")
	// After 2:1 grouping: Qty = 100, PM = 20, TotalCost = 2000 (unchanged)
	if !pos.Quantity.Equal(d("100")) {
		t.Errorf("Qty after grouping = %s, want 100", pos.Quantity)
	}
	if !pos.AveragePrice.Equal(d("20")) {
		t.Errorf("PM after grouping = %s, want 20", pos.AveragePrice)
	}
}

func TestAveragePriceEngine_Bonus(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("3000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			// Bonus: 10 shares at price 0
			ID: "2", AssetID: "asset-1", OperationType: domain.OpBonus,
			Quantity: d("10"), UnitPrice: d("0"), Costs: d("0"),
			TotalAmount: d("0"), OperationDate: makeDate(2024, 6, 1),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := engine.GetPosition("asset-1")
	// After bonus: Qty = 110, TotalCost = 3000, PM = 3000/110 = 27.2727...
	expectedPM := d("3000").Div(d("110"))
	if !pos.Quantity.Equal(d("110")) {
		t.Errorf("Qty after bonus = %s, want 110", pos.Quantity)
	}
	if !pos.AveragePrice.Equal(expectedPM) {
		t.Errorf("PM after bonus = %s, want %s", pos.AveragePrice, expectedPM)
	}
}

func TestAveragePriceEngine_Amortization(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "fii-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("100.00"), Costs: d("0"),
			TotalAmount: d("10000.00"), OperationDate: makeDate(2024, 1, 10),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Apply amortization of R$ 5.00 per unit
	engine.ApplyAmortization("fii-1", d("5.00"))

	pos := engine.GetPosition("fii-1")
	// PM = 100 - 5 = 95
	if !pos.AveragePrice.Equal(d("95")) {
		t.Errorf("PM after amortization = %s, want 95", pos.AveragePrice)
	}
	if !pos.TotalCost.Equal(d("9500")) {
		t.Errorf("TotalCost after amortization = %s, want 9500", pos.TotalCost)
	}
}

func TestAveragePriceEngine_SellInsufficientQty(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("50"), UnitPrice: d("20.00"), Costs: d("0"),
			TotalAmount: d("1000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "asset-1", OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("25.00"), Costs: d("0"),
			TotalAmount: d("2500.00"), OperationDate: makeDate(2024, 3, 10),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err == nil {
		t.Fatal("expected error for insufficient quantity, got nil")
	}
}

func TestAveragePriceEngine_MultipleAssets(t *testing.T) {
	engine := NewAveragePriceEngine()

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "PETR4", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("3000.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "VALE3", OperationType: domain.OpBuy,
			Quantity: d("200"), UnitPrice: d("50.00"), Costs: d("0"),
			TotalAmount: d("10000.00"), OperationDate: makeDate(2024, 1, 15),
		},
	}

	_, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	petr := engine.GetPosition("PETR4")
	vale := engine.GetPosition("VALE3")

	if !petr.AveragePrice.Equal(d("30")) {
		t.Errorf("PETR4 PM = %s, want 30", petr.AveragePrice)
	}
	if !vale.AveragePrice.Equal(d("50")) {
		t.Errorf("VALE3 PM = %s, want 50", vale.AveragePrice)
	}
}

func TestAveragePriceEngine_ChronologicalSorting(t *testing.T) {
	engine := NewAveragePriceEngine()

	// Transactions given out of order — engine must sort them
	txs := []domain.Transaction{
		{
			ID: "2", AssetID: "asset-1", OperationType: domain.OpSell,
			Quantity: d("50"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("1500.00"), OperationDate: makeDate(2024, 3, 10),
		},
		{
			ID: "1", AssetID: "asset-1", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("20.00"), Costs: d("0"),
			TotalAmount: d("2000.00"), OperationDate: makeDate(2024, 1, 10),
		},
	}

	results, err := engine.ProcessTransactions(txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 sale result, got %d", len(results))
	}

	// Should process buy first, then sell
	r := results[0]
	if !r.ProfitLoss.Equal(d("500")) {
		t.Errorf("ProfitLoss = %s, want 500", r.ProfitLoss)
	}
}
