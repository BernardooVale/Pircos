package service

import (
	"testing"
	"time"

	"github.com/pircos/api/internal/domain"
)

func TestClassifyTrades_NoDayTrade(t *testing.T) {
	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "PETR4", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("25.00"), Costs: d("0"),
			TotalAmount: d("2500.00"), OperationDate: makeDate(2024, 1, 10),
		},
		{
			ID: "2", AssetID: "PETR4", OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("30.00"), Costs: d("0"),
			TotalAmount: d("3000.00"), OperationDate: makeDate(2024, 2, 15),
		},
	}

	dtPairs, swingTxs := ClassifyTrades(txs)

	if len(dtPairs) != 0 {
		t.Errorf("expected 0 day trade pairs, got %d", len(dtPairs))
	}
	if len(swingTxs) != 2 {
		t.Errorf("expected 2 swing transactions (1 buy, 1 sell), got %d", len(swingTxs))
	}
}

func TestClassifyTrades_FullDayTrade(t *testing.T) {
	sameDay := makeDate(2024, 3, 15)

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "PETR4", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("25.00"), Costs: d("5.00"),
			TotalAmount: d("2505.00"), OperationDate: sameDay,
		},
		{
			ID: "2", AssetID: "PETR4", OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("27.00"), Costs: d("5.00"),
			TotalAmount: d("2695.00"),
			OperationDate: sameDay.Add(2 * time.Hour),
		},
	}

	dtPairs, swingTxs := ClassifyTrades(txs)

	if len(dtPairs) != 1 {
		t.Fatalf("expected 1 day trade pair, got %d", len(dtPairs))
	}
	if len(swingTxs) != 0 {
		t.Errorf("expected 0 swing transactions, got %d", len(swingTxs))
	}

	pair := dtPairs[0]
	if !pair.Quantity.Equal(d("100")) {
		t.Errorf("matched qty = %s, want 100", pair.Quantity)
	}
}

func TestClassifyTrades_PartialDayTrade(t *testing.T) {
	sameDay := makeDate(2024, 3, 15)

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "PETR4", OperationType: domain.OpBuy,
			Quantity: d("50"), UnitPrice: d("25.00"), Costs: d("5.00"),
			TotalAmount: d("1255.00"), OperationDate: sameDay,
		},
		{
			ID: "2", AssetID: "PETR4", OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("27.00"), Costs: d("10.00"),
			TotalAmount: d("2690.00"),
			OperationDate: sameDay.Add(3 * time.Hour),
		},
	}

	dtPairs, swingTxs := ClassifyTrades(txs)

	if len(dtPairs) != 1 {
		t.Fatalf("expected 1 day trade pair, got %d", len(dtPairs))
	}

	pair := dtPairs[0]
	// Only 50 units matched as day trade (buy qty = 50)
	if !pair.Quantity.Equal(d("50")) {
		t.Errorf("matched qty = %s, want 50", pair.Quantity)
	}

	// Remaining 50 units should be swing trade sell
	if len(swingTxs) != 1 {
		t.Fatalf("expected 1 swing transaction, got %d", len(swingTxs))
	}
	if !swingTxs[0].Quantity.Equal(d("50")) {
		t.Errorf("swing sale qty = %s, want 50", swingTxs[0].Quantity)
	}
}

func TestClassifyTrades_DifferentAssets(t *testing.T) {
	sameDay := makeDate(2024, 3, 15)

	txs := []domain.Transaction{
		{
			ID: "1", AssetID: "PETR4", OperationType: domain.OpBuy,
			Quantity: d("100"), UnitPrice: d("25.00"), Costs: d("0"),
			TotalAmount: d("2500.00"), OperationDate: sameDay,
		},
		{
			ID: "2", AssetID: "VALE3", OperationType: domain.OpSell,
			Quantity: d("100"), UnitPrice: d("50.00"), Costs: d("0"),
			TotalAmount: d("5000.00"), OperationDate: sameDay,
		},
	}

	dtPairs, swingTxs := ClassifyTrades(txs)

	// Different assets => no day trade
	if len(dtPairs) != 0 {
		t.Errorf("expected 0 day trade pairs, got %d", len(dtPairs))
	}
	if len(swingTxs) != 2 {
		t.Errorf("expected 2 swing transactions (1 buy, 1 sell), got %d", len(swingTxs))
	}
}

func TestDayTradeProfitLoss(t *testing.T) {
	pairs := []TradePair{
		{
			BuyTx: domain.Transaction{
				Quantity: d("100"), UnitPrice: d("25.00"), Costs: d("5.00"),
			},
			SellTx: domain.Transaction{
				Quantity: d("100"), UnitPrice: d("27.00"), Costs: d("5.00"),
			},
			Quantity: d("100"),
		},
	}

	pl := DayTradeProfitLoss(pairs)
	// P/L = (27*100) - (25*100) - 5 - 5 = 2700 - 2500 - 10 = 190
	expected := d("190")
	if !pl.Equal(expected) {
		t.Errorf("DayTradeProfitLoss = %s, want %s", pl, expected)
	}
}

func TestDayTradeTotalSales(t *testing.T) {
	pairs := []TradePair{
		{
			SellTx:  domain.Transaction{UnitPrice: d("27.00"), Quantity: d("100")},
			Quantity: d("100"),
		},
		{
			SellTx:  domain.Transaction{UnitPrice: d("30.00"), Quantity: d("50")},
			Quantity: d("50"),
		},
	}

	total := DayTradeTotalSales(pairs)
	// 27*100 + 30*50 = 2700 + 1500 = 4200
	if !total.Equal(d("4200")) {
		t.Errorf("DayTradeTotalSales = %s, want 4200", total)
	}
}

func TestToFiscalDate(t *testing.T) {
	dt := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	result := toFiscalDate(dt)
	if result != "2024-03-15" {
		t.Errorf("toFiscalDate = %s, want 2024-03-15", result)
	}
}
