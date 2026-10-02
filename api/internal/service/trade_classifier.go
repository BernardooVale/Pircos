package service

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// ClassifiedTrade represents a trade that has been classified as day trade or swing trade.
type ClassifiedTrade struct {
	Transaction domain.Transaction
	IsDayTrade  bool
	// For day trades, the matched quantity (a single buy may be partially day trade)
	DayTradeQty  decimal.Decimal
	SwingTradeQty decimal.Decimal
}

// TradePair represents a matched buy-sell pair for day trade detection.
type TradePair struct {
	BuyTx   domain.Transaction
	SellTx  domain.Transaction
	Quantity decimal.Decimal // matched quantity
}

// ClassifyTrades identifies day trades within a set of transactions.
// A day trade occurs when there is a buy and sell of the same asset on the same
// fiscal date (DATE(operation_date)).
//
// Returns:
//   - dayTradePairs: matched buy-sell pairs that constitute day trades
//   - swingTxs: all remaining transactions (buys, sells, splits, groupings, etc.) for swing trade processing
func ClassifyTrades(transactions []domain.Transaction) (dayTradePairs []TradePair, swingTxs []domain.Transaction) {
	// Group transactions by asset_id and fiscal date
	type dateAssetKey struct {
		AssetID    string
		FiscalDate string // YYYY-MM-DD
	}

	groups := make(map[dateAssetKey][]domain.Transaction)
	var otherTxs []domain.Transaction

	for _, tx := range transactions {
		if tx.OperationType != domain.OpBuy && tx.OperationType != domain.OpSell {
			otherTxs = append(otherTxs, tx)
			continue
		}
		key := dateAssetKey{
			AssetID:    tx.AssetID,
			FiscalDate: toFiscalDate(tx.OperationDate),
		}
		groups[key] = append(groups[key], tx)
	}

	for _, txGroup := range groups {
		var buys, sells []domain.Transaction
		for _, tx := range txGroup {
			switch tx.OperationType {
			case domain.OpBuy:
				buys = append(buys, tx)
			case domain.OpSell:
				sells = append(sells, tx)
			}
		}

		if len(buys) == 0 {
			swingTxs = append(swingTxs, sells...)
			continue
		}
		if len(sells) == 0 {
			swingTxs = append(swingTxs, buys...)
			continue
		}

		// Sort buys and sells by time within the day
		sort.Slice(buys, func(i, j int) bool {
			return buys[i].OperationDate.Before(buys[j].OperationDate)
		})
		sort.Slice(sells, func(i, j int) bool {
			return sells[i].OperationDate.Before(sells[j].OperationDate)
		})

		// Match day trades FIFO
		pairs, remainingBuys, remainingSells := matchDayTrades(buys, sells)
		dayTradePairs = append(dayTradePairs, pairs...)
		swingTxs = append(swingTxs, remainingBuys...)
		swingTxs = append(swingTxs, remainingSells...)
	}

	// Include non-buy/sell transactions for processing
	swingTxs = append(swingTxs, otherTxs...)

	return dayTradePairs, swingTxs
}

// matchDayTrades matches buys with sells on the same day using FIFO.
// Returns matched day trade pairs and any remaining unmatched buy and sell transactions
// (which are swing trades).
func matchDayTrades(buys, sells []domain.Transaction) ([]TradePair, []domain.Transaction, []domain.Transaction) {
	var pairs []TradePair
	var remainingBuys []domain.Transaction
	var remainingSells []domain.Transaction

	// Track remaining quantity for each buy
	buyRemaining := make([]decimal.Decimal, len(buys))
	for i, b := range buys {
		buyRemaining[i] = b.Quantity
	}

	for _, sell := range sells {
		sellRemaining := sell.Quantity

		for i := range buys {
			if buyRemaining[i].IsZero() {
				continue
			}
			if sellRemaining.IsZero() {
				break
			}

			// Match the minimum of remaining buy and sell quantities
			matchQty := decimal.Min(buyRemaining[i], sellRemaining)

			pairs = append(pairs, TradePair{
				BuyTx:    buys[i],
				SellTx:   sell,
				Quantity: matchQty,
			})

			buyRemaining[i] = buyRemaining[i].Sub(matchQty)
			sellRemaining = sellRemaining.Sub(matchQty)
		}

		// Any unmatched sell quantity is a swing trade
		if sellRemaining.IsPositive() {
			swingSell := sell
			swingSell.Quantity = sellRemaining
			swingSell.TotalAmount = sell.UnitPrice.Mul(sellRemaining)
			if sell.Quantity.IsPositive() {
				costRatio := sellRemaining.Div(sell.Quantity)
				swingSell.Costs = sell.Costs.Mul(costRatio)
			}
			remainingSells = append(remainingSells, swingSell)
		}
	}

	// Any unmatched buy quantity is also a swing trade
	for i, rem := range buyRemaining {
		if rem.IsPositive() {
			swingBuy := buys[i]
			swingBuy.Quantity = rem
			swingBuy.TotalAmount = buys[i].UnitPrice.Mul(rem)
			if buys[i].Quantity.IsPositive() {
				costRatio := rem.Div(buys[i].Quantity)
				swingBuy.Costs = buys[i].Costs.Mul(costRatio)
			}
			remainingBuys = append(remainingBuys, swingBuy)
		}
	}

	return pairs, remainingBuys, remainingSells
}

// DayTradeProfitLoss calculates the profit/loss for a set of day trade pairs.
func DayTradeProfitLoss(pairs []TradePair) decimal.Decimal {
	total := decimal.Zero
	for _, p := range pairs {
		sellValue := p.SellTx.UnitPrice.Mul(p.Quantity)
		buyValue := p.BuyTx.UnitPrice.Mul(p.Quantity)

		// Prorate costs proportionally
		var sellCosts, buyCosts decimal.Decimal
		if p.SellTx.Quantity.IsPositive() {
			sellCosts = p.SellTx.Costs.Mul(p.Quantity).Div(p.SellTx.Quantity)
		}
		if p.BuyTx.Quantity.IsPositive() {
			buyCosts = p.BuyTx.Costs.Mul(p.Quantity).Div(p.BuyTx.Quantity)
		}

		pl := sellValue.Sub(buyValue).Sub(sellCosts).Sub(buyCosts)
		total = total.Add(pl)
	}
	return total
}

// DayTradeTotalSales returns the total sales volume from day trade pairs.
func DayTradeTotalSales(pairs []TradePair) decimal.Decimal {
	total := decimal.Zero
	for _, p := range pairs {
		total = total.Add(p.SellTx.UnitPrice.Mul(p.Quantity))
	}
	return total
}

// toFiscalDate converts a timestamp to a fiscal date string YYYY-MM-DD.
func toFiscalDate(t time.Time) string {
	return t.Format("2006-01-02")
}
