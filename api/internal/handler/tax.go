package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
	"github.com/pircos/api/internal/service"
)

// TaxHandler handles tax overview and drill-down endpoints.
type TaxHandler struct {
	balanceRepo *repository.MonthlyTaxBalanceRepo
	txRepo      *repository.TransactionRepo
	assetRepo   *repository.AssetRepo
	userRepo    *repository.UserRepo
}

// NewTaxHandler creates a new TaxHandler.
func NewTaxHandler(
	balanceRepo *repository.MonthlyTaxBalanceRepo,
	txRepo *repository.TransactionRepo,
	assetRepo *repository.AssetRepo,
	userRepo *repository.UserRepo,
) *TaxHandler {
	return &TaxHandler{
		balanceRepo: balanceRepo,
		txRepo:      txRepo,
		assetRepo:   assetRepo,
		userRepo:    userRepo,
	}
}

// MonthCard represents one month in the annual IR preview.
type MonthCard struct {
	YearMonth   string                     `json:"year_month"`
	MonthNumber int                        `json:"month_number"`
	TotalSales  decimal.Decimal            `json:"total_sales"`
	TaxableBase decimal.Decimal            `json:"taxable_base"`
	TaxDue      decimal.Decimal            `json:"tax_due"`
	FinalDARF   decimal.Decimal            `json:"final_darf"`
	Status      string                     `json:"status"` // "DUE", "EXEMPT", "NO_ACTIVITY"
	Buckets     []domain.MonthlyTaxBalance `json:"buckets"`
}

// MonthlyPreviewResponse represents the response for GET /api/v1/tax/monthly
type MonthlyPreviewResponse struct {
	Year        string          `json:"year"`
	TotalTaxDue decimal.Decimal `json:"total_tax_due"`
	TotalDARF   decimal.Decimal `json:"total_darf"`
	Months      []MonthCard     `json:"months"`
}

// Monthly handles GET /api/v1/tax/monthly?year=YYYY
func (h *TaxHandler) Monthly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	year := q.Get("year")
	if year == "" {
		year = fmt.Sprintf("%d", time.Now().Year())
	}

	userID := q.Get("user_id")
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	balances, err := h.balanceRepo.GetByUserAndYear(ctx, userID, year)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed querying tax balances: "+err.Error())
		return
	}

	// Group balances by month (1 to 12)
	balancesByMonth := make(map[string][]domain.MonthlyTaxBalance)
	for _, b := range balances {
		balancesByMonth[b.YearMonth] = append(balancesByMonth[b.YearMonth], b)
	}

	totalYearTax := decimal.Zero
	totalYearDARF := decimal.Zero
	var monthCards []MonthCard

	for m := 1; m <= 12; m++ {
		ym := fmt.Sprintf("%s-%02d", year, m)
		bucketList := balancesByMonth[ym]

		mSales := decimal.Zero
		mTaxable := decimal.Zero
		mTaxDue := decimal.Zero
		mDARF := decimal.Zero

		for _, b := range bucketList {
			mSales = mSales.Add(b.TotalSales)
			mTaxable = mTaxable.Add(b.TaxableBase)
			mTaxDue = mTaxDue.Add(b.TaxDue)
			mDARF = mDARF.Add(b.FinalDARF)
		}

		status := "NO_ACTIVITY"
		if mDARF.GreaterThan(decimal.NewFromInt(10)) {
			// DARF below R$ 10.00 is carried to next month per Brazilian tax rule
			status = "DUE"
		} else if mSales.IsPositive() && mDARF.IsZero() {
			status = "EXEMPT"
		}

		totalYearTax = totalYearTax.Add(mTaxDue)
		totalYearDARF = totalYearDARF.Add(mDARF)

		monthCards = append(monthCards, MonthCard{
			YearMonth:   ym,
			MonthNumber: m,
			TotalSales:  mSales,
			TaxableBase: mTaxable,
			TaxDue:      mTaxDue,
			FinalDARF:   mDARF,
			Status:      status,
			Buckets:     bucketList,
		})
	}

	writeJSON(w, http.StatusOK, MonthlyPreviewResponse{
		Year:        year,
		TotalTaxDue: totalYearTax,
		TotalDARF:   totalYearDARF,
		Months:      monthCards,
	})
}

// DrilldownSaleItem represents a single sale operation in the drilldown view.
type DrilldownSaleItem struct {
	TransactionID string          `json:"transaction_id"`
	Ticker        string          `json:"ticker"`
	AssetName     string          `json:"asset_name"`
	OperationDate time.Time       `json:"operation_date"`
	Quantity      decimal.Decimal `json:"quantity"`
	UnitPrice     decimal.Decimal `json:"unit_price"`
	SaleAmount    decimal.Decimal `json:"sale_amount"`
	Costs         decimal.Decimal `json:"costs"`
	ProfitLoss    decimal.Decimal `json:"profit_loss"`
}

// DrilldownResponse represents the detailed view for GET /api/v1/tax/drilldown
type DrilldownResponse struct {
	YearMonth string                    `json:"year_month"`
	Bucket    string                    `json:"bucket"`
	Balance   *domain.MonthlyTaxBalance `json:"balance,omitempty"`
	Sales     []DrilldownSaleItem       `json:"sales"`
}

// Drilldown handles GET /api/v1/tax/drilldown?year_month=YYYY-MM&bucket=BUCKET
func (h *TaxHandler) Drilldown(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	yearMonth := q.Get("year_month")
	bucket := q.Get("bucket")
	if yearMonth == "" || bucket == "" {
		writeError(w, http.StatusBadRequest, "year_month and bucket parameters are required")
		return
	}

	userID := q.Get("user_id")
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	// 1. Get snapshot balance
	balance, err := h.balanceRepo.GetByUserMonthBucket(ctx, userID, yearMonth, bucket)
	if err != nil {
		// Non-fatal if no balance recorded yet
		balance = nil
	}

	// 2. Fetch all transactions for this month to reconstruct sales details
	startMonth, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid year_month format (use YYYY-MM)")
		return
	}
	endMonth := startMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	allTxs, err := h.txRepo.GetByUserID(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed fetching transactions: "+err.Error())
		return
	}

	assetMap, _ := h.assetRepo.GetAllMap(ctx)

	// Filter and compute sale details for both swing and day trades
	dtPairs, swingTxs := service.ClassifyTrades(allTxs)
	pmEngine := service.NewAveragePriceEngine()
	saleResults, _ := pmEngine.ProcessTransactions(swingTxs)

	var sales []DrilldownSaleItem

	// Process swing trade sales
	for _, sr := range saleResults {
		if sr.OperationDate.Before(startMonth) || sr.OperationDate.After(endMonth) {
			continue
		}

		asset := assetMap[sr.AssetID]
		assetBucket := service.GetBucketForAsset(asset.AssetClass, false)
		if assetBucket != bucket {
			continue
		}

		// Find original transaction to get costs
		var txCosts decimal.Decimal
		for _, tx := range allTxs {
			if tx.ID == sr.TransactionID {
				txCosts = tx.Costs
				break
			}
		}

		sales = append(sales, DrilldownSaleItem{
			TransactionID: sr.TransactionID,
			Ticker:        asset.Ticker,
			AssetName:     asset.Name,
			OperationDate: sr.OperationDate,
			Quantity:      sr.Quantity,
			UnitPrice:     sr.SaleAmount.Add(txCosts).Div(sr.Quantity),
			SaleAmount:    sr.SaleAmount,
			Costs:         txCosts,
			ProfitLoss:    sr.ProfitLoss,
		})
	}

	// Process day trade pairs
	for _, pair := range dtPairs {
		if pair.SellTx.OperationDate.Before(startMonth) || pair.SellTx.OperationDate.After(endMonth) {
			continue
		}

		asset := assetMap[pair.SellTx.AssetID]
		assetBucket := service.GetBucketForAsset(asset.AssetClass, true)
		if assetBucket != bucket {
			continue
		}

		dtSales := service.DayTradeTotalSales([]service.TradePair{pair})
		dtPL := service.DayTradeProfitLoss([]service.TradePair{pair})
		dtCosts := pair.SellTx.Costs.Add(pair.BuyTx.Costs)

		sales = append(sales, DrilldownSaleItem{
			TransactionID: pair.SellTx.ID,
			Ticker:        asset.Ticker,
			AssetName:     asset.Name + " (Day Trade)",
			OperationDate: pair.SellTx.OperationDate,
			Quantity:      pair.Quantity,
			UnitPrice:     pair.SellTx.UnitPrice,
			SaleAmount:    dtSales,
			Costs:         dtCosts,
			ProfitLoss:    dtPL,
		})
	}

	if sales == nil {
		sales = []DrilldownSaleItem{}
	}

	writeJSON(w, http.StatusOK, DrilldownResponse{
		YearMonth: yearMonth,
		Bucket:    bucket,
		Balance:   balance,
		Sales:     sales,
	})
}
