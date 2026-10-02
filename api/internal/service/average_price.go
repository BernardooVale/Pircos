package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// Position represents the current holding of an asset for a given user.
type Position struct {
	AssetID      string
	Quantity     decimal.Decimal
	AveragePrice decimal.Decimal // Preço médio ponderado
	TotalCost    decimal.Decimal // Custo total acumulado (Qtd × PM)
}

// SaleResult represents the profit/loss result of a sell operation.
type SaleResult struct {
	TransactionID string
	AssetID       string
	Quantity      decimal.Decimal
	SaleAmount    decimal.Decimal // Valor bruto da venda - custos
	CostBasis     decimal.Decimal // Qtd vendida × PM
	ProfitLoss    decimal.Decimal // SaleAmount - CostBasis
	OperationDate time.Time
	IsDayTrade    bool
}

// AveragePriceEngine calculates weighted average prices and tracks positions
// for a set of transactions, following Brazilian tax accounting rules.
//
// Rules (from AGENT_SPEC section 4.1):
//   - BUY: New PM = ((Qty * PM) + (BuyQty * UnitPrice) + Costs) / (Qty + BuyQty)
//   - SELL: PM does not change. P/L = (GrossSale - Costs) - (SoldQty * PM)
//   - AMORTIZATION (FII): Amortized value per unit reduces PM directly.
//   - SPLIT: Quantity increases, PM decreases proportionally (total cost unchanged).
//   - GROUPING: Quantity decreases, PM increases proportionally (total cost unchanged).
//   - BONUS: Treated as a buy at price 0 (dilutes PM).
type AveragePriceEngine struct {
	positions map[string]*Position // keyed by asset_id
}

// NewAveragePriceEngine creates a new engine instance.
func NewAveragePriceEngine() *AveragePriceEngine {
	return &AveragePriceEngine{
		positions: make(map[string]*Position),
	}
}

// ProcessTransactions processes a list of transactions in chronological order,
// computing positions and sale results.
// Transactions are sorted by operation_date before processing.
func (e *AveragePriceEngine) ProcessTransactions(transactions []domain.Transaction) ([]SaleResult, error) {
	// Sort by operation date (chronological order is critical)
	sorted := make([]domain.Transaction, len(transactions))
	copy(sorted, transactions)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].OperationDate.Equal(sorted[j].OperationDate) {
			// Buys before sells on the same timestamp
			if sorted[i].OperationType == domain.OpBuy && sorted[j].OperationType == domain.OpSell {
				return true
			}
			return false
		}
		return sorted[i].OperationDate.Before(sorted[j].OperationDate)
	})

	var saleResults []SaleResult

	for _, tx := range sorted {
		switch tx.OperationType {
		case domain.OpBuy:
			e.processBuy(tx)
		case domain.OpSell:
			result, err := e.processSell(tx)
			if err != nil {
				return nil, fmt.Errorf("processing sell %s: %w", tx.ID, err)
			}
			saleResults = append(saleResults, *result)
		case domain.OpSplit:
			e.processSplit(tx)
		case domain.OpGrouping:
			e.processGrouping(tx)
		case domain.OpBonus:
			e.processBonus(tx)
		default:
			return nil, fmt.Errorf("unknown operation type: %s", tx.OperationType)
		}
	}

	return saleResults, nil
}

// processBuy handles a BUY operation.
// New PM = ((CurrentQty * CurrentPM) + (BuyQty * UnitPrice) + Costs) / (CurrentQty + BuyQty)
func (e *AveragePriceEngine) processBuy(tx domain.Transaction) {
	pos := e.getOrCreatePosition(tx.AssetID)

	currentTotal := pos.Quantity.Mul(pos.AveragePrice)
	buyTotal := tx.Quantity.Mul(tx.UnitPrice).Add(tx.Costs)
	newQuantity := pos.Quantity.Add(tx.Quantity)

	if newQuantity.IsPositive() {
		pos.AveragePrice = currentTotal.Add(buyTotal).Div(newQuantity)
	}
	pos.Quantity = newQuantity
	pos.TotalCost = pos.Quantity.Mul(pos.AveragePrice)
}

// processSell handles a SELL operation.
// PM does not change. P/L = (GrossSale - Costs) - (SoldQty * PM)
func (e *AveragePriceEngine) processSell(tx domain.Transaction) (*SaleResult, error) {
	pos := e.getOrCreatePosition(tx.AssetID)

	if pos.Quantity.LessThan(tx.Quantity) {
		return nil, fmt.Errorf("insufficient quantity for asset %s: have %s, selling %s",
			tx.AssetID, pos.Quantity.String(), tx.Quantity.String())
	}

	// Net sale amount = total_amount - costs (total_amount already includes the gross)
	netSaleAmount := tx.TotalAmount.Sub(tx.Costs)

	// Cost basis for the sold quantity
	costBasis := tx.Quantity.Mul(pos.AveragePrice)

	// Profit/Loss
	profitLoss := netSaleAmount.Sub(costBasis)

	// Reduce quantity (PM stays the same)
	pos.Quantity = pos.Quantity.Sub(tx.Quantity)
	pos.TotalCost = pos.Quantity.Mul(pos.AveragePrice)

	// If position is fully closed, reset PM
	if pos.Quantity.IsZero() {
		pos.AveragePrice = decimal.Zero
		pos.TotalCost = decimal.Zero
	}

	return &SaleResult{
		TransactionID: tx.ID,
		AssetID:       tx.AssetID,
		Quantity:      tx.Quantity,
		SaleAmount:    netSaleAmount,
		CostBasis:     costBasis,
		ProfitLoss:    profitLoss,
		OperationDate: tx.OperationDate,
	}, nil
}

// processSplit handles a SPLIT operation.
// Quantity increases, PM decreases proportionally. Total cost unchanged.
// tx.Quantity = new total quantity after split
// tx.UnitPrice = split ratio (e.g., 2 means 1:2 split)
func (e *AveragePriceEngine) processSplit(tx domain.Transaction) {
	pos := e.getOrCreatePosition(tx.AssetID)
	if pos.Quantity.IsZero() {
		return
	}

	ratio := tx.UnitPrice
	if ratio.IsZero() || ratio.Equal(decimal.NewFromInt(1)) {
		return
	}

	totalCost := pos.Quantity.Mul(pos.AveragePrice)
	pos.Quantity = pos.Quantity.Mul(ratio)
	if pos.Quantity.IsPositive() {
		pos.AveragePrice = totalCost.Div(pos.Quantity)
	}
	pos.TotalCost = totalCost // unchanged
}

// processGrouping handles a GROUPING (reverse split) operation.
// Quantity decreases, PM increases proportionally. Total cost unchanged.
// tx.UnitPrice = grouping ratio (e.g., 0.5 means 2:1 grouping)
func (e *AveragePriceEngine) processGrouping(tx domain.Transaction) {
	pos := e.getOrCreatePosition(tx.AssetID)
	if pos.Quantity.IsZero() {
		return
	}

	ratio := tx.UnitPrice
	if ratio.IsZero() || ratio.Equal(decimal.NewFromInt(1)) {
		return
	}

	totalCost := pos.Quantity.Mul(pos.AveragePrice)
	pos.Quantity = pos.Quantity.Mul(ratio)
	if pos.Quantity.IsPositive() {
		pos.AveragePrice = totalCost.Div(pos.Quantity)
	}
	pos.TotalCost = totalCost // unchanged
}

// processBonus handles a BONUS operation.
// Treated as a buy at price 0 — dilutes the average price.
func (e *AveragePriceEngine) processBonus(tx domain.Transaction) {
	pos := e.getOrCreatePosition(tx.AssetID)

	currentTotal := pos.Quantity.Mul(pos.AveragePrice)
	newQuantity := pos.Quantity.Add(tx.Quantity)

	if newQuantity.IsPositive() {
		pos.AveragePrice = currentTotal.Div(newQuantity)
	}
	pos.Quantity = newQuantity
	pos.TotalCost = pos.Quantity.Mul(pos.AveragePrice)
}

// ApplyAmortization reduces the average price by the amortized value per unit.
// Used for FII amortization events.
func (e *AveragePriceEngine) ApplyAmortization(assetID string, amortPerUnit decimal.Decimal) {
	pos := e.getOrCreatePosition(assetID)
	if pos.Quantity.IsZero() {
		return
	}

	pos.AveragePrice = pos.AveragePrice.Sub(amortPerUnit)
	if pos.AveragePrice.IsNegative() {
		pos.AveragePrice = decimal.Zero
	}
	pos.TotalCost = pos.Quantity.Mul(pos.AveragePrice)
}

// GetPosition returns the current position for an asset.
func (e *AveragePriceEngine) GetPosition(assetID string) *Position {
	if pos, ok := e.positions[assetID]; ok {
		return pos
	}
	return &Position{
		AssetID:      assetID,
		Quantity:     decimal.Zero,
		AveragePrice: decimal.Zero,
		TotalCost:    decimal.Zero,
	}
}

// GetAllPositions returns all non-zero positions.
func (e *AveragePriceEngine) GetAllPositions() []*Position {
	var result []*Position
	for _, pos := range e.positions {
		if pos.Quantity.IsPositive() {
			result = append(result, pos)
		}
	}
	return result
}

func (e *AveragePriceEngine) getOrCreatePosition(assetID string) *Position {
	if pos, ok := e.positions[assetID]; ok {
		return pos
	}
	pos := &Position{
		AssetID:      assetID,
		Quantity:     decimal.Zero,
		AveragePrice: decimal.Zero,
		TotalCost:    decimal.Zero,
	}
	e.positions[assetID] = pos
	return pos
}
