package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

func TestParseBRDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected decimal.Decimal
		wantErr  bool
	}{
		{"1.234,56", d("1234.56"), false},
		{"34,50", d("34.50"), false},
		{"0,18", d("0.18"), false},
		{"1000", d("1000"), false},
		{"0,00", d("0"), false},
		{"invalid", decimal.Zero, true},
	}

	for _, tt := range tests {
		got, err := parseBRDecimal(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseBRDecimal(%q) expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseBRDecimal(%q) unexpected error: %v", tt.input, err)
		}
		if !got.Equal(tt.expected) {
			t.Errorf("parseBRDecimal(%q) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestSinacorParser_ParseText(t *testing.T) {
	sampleSinacorText := `
NOTA DE CORRETAGEM
Nr. nota: 987654
Data pregão: 15/03/2024
Líquido para: 19/03/2024

Negociação C/V Tipo mercado Prazo Especificação do título Obs. (*) Quantidade Preço / Ajuste Valor / Ajuste D/C
1-BOVESPA C VISTA PETR4 PETROBRAS PN 100 34,50 3.450,00 D
1-BOVESPA V VISTA VALE3 VALE ON NM 200 65,00 13.000,00 C

Resumo dos Negócios
Total CBLC: 0,00
Taxa de liquidação: 1,50
Emolumentos: 0,50
Corretagem: 8,00
Total custos / despesas: 10,00
I.R.R.F. Day Trade: 0,00
`

	parser := NewSinacorParser()
	note, err := parser.ParseText(sampleSinacorText)
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}

	// Verify Header
	if note.NoteNumber != "987654" {
		t.Errorf("NoteNumber = %s, want 987654", note.NoteNumber)
	}
	expectedOpDate := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !note.OperationDate.Equal(expectedOpDate) {
		t.Errorf("OperationDate = %v, want %v", note.OperationDate, expectedOpDate)
	}
	if note.SettlementDate == nil {
		t.Fatal("expected settlement date, got nil")
	}
	expectedSetDate := time.Date(2024, 3, 19, 0, 0, 0, 0, time.UTC)
	if !note.SettlementDate.Equal(expectedSetDate) {
		t.Errorf("SettlementDate = %v, want %v", *note.SettlementDate, expectedSetDate)
	}

	// Verify Costs
	if !note.TotalCosts.Equal(d("10.00")) {
		t.Errorf("TotalCosts = %s, want 10.00", note.TotalCosts)
	}

	// Verify Operations
	if len(note.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(note.Transactions))
	}

	// PETR4 buy
	op1 := note.Transactions[0]
	if op1.OperationType != domain.OpBuy {
		t.Errorf("op1 OperationType = %s, want BUY", op1.OperationType)
	}
	if op1.Ticker != "PETR4" {
		t.Errorf("op1 Ticker = %s, want PETR4", op1.Ticker)
	}
	if !op1.Quantity.Equal(d("100")) {
		t.Errorf("op1 Quantity = %s, want 100", op1.Quantity)
	}
	if !op1.UnitPrice.Equal(d("34.50")) {
		t.Errorf("op1 UnitPrice = %s, want 34.50", op1.UnitPrice)
	}
	if !op1.TotalAmount.Equal(d("3450.00")) {
		t.Errorf("op1 TotalAmount = %s, want 3450.00", op1.TotalAmount)
	}

	// VALE3 sell
	op2 := note.Transactions[1]
	if op2.OperationType != domain.OpSell {
		t.Errorf("op2 OperationType = %s, want SELL", op2.OperationType)
	}
	if op2.Ticker != "VALE3" {
		t.Errorf("op2 Ticker = %s, want VALE3", op2.Ticker)
	}
	if !op2.Quantity.Equal(d("200")) {
		t.Errorf("op2 Quantity = %s, want 200", op2.Quantity)
	}
	if !op2.UnitPrice.Equal(d("65.00")) {
		t.Errorf("op2 UnitPrice = %s, want 65.00", op2.UnitPrice)
	}
	if !op2.TotalAmount.Equal(d("13000.00")) {
		t.Errorf("op2 TotalAmount = %s, want 13000.00", op2.TotalAmount)
	}

	// Verify Cost Proration:
	// Total volume = 3450 + 13000 = 16450
	// Total costs = 10.00
	// op1 cost = 10 * 3450 / 16450 = 2.10
	// op2 cost = 10 * 13000 / 16450 = 7.90
	totalProrated := op1.Costs.Add(op2.Costs)
	if !totalProrated.Equal(note.TotalCosts) {
		t.Errorf("total prorated costs = %s, want %s (op1=%s, op2=%s)",
			totalProrated, note.TotalCosts, op1.Costs, op2.Costs)
	}
	if !op1.Costs.Equal(d("2.10")) {
		t.Errorf("op1 Costs = %s, want 2.10", op1.Costs)
	}
	if !op2.Costs.Equal(d("7.90")) {
		t.Errorf("op2 Costs = %s, want 7.90", op2.Costs)
	}
}
