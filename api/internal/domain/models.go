package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// User represents a system user (prepared for multi-tenant).
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Asset represents a financial asset (stock, FII, ETF, crypto, etc.).
type Asset struct {
	ID         string    `json:"id"`
	Ticker     string    `json:"ticker"`
	Name       string    `json:"name"`
	AssetClass string    `json:"asset_class"`
	Currency   string    `json:"currency"`
	CreatedAt  time.Time `json:"created_at"`
}

// Document represents an uploaded PDF brokerage note.
type Document struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	OriginalFilename string    `json:"original_filename"`
	StoragePath      string    `json:"storage_path"`
	FileSizeBytes    int64     `json:"file_size_bytes"`
	MimeType         string    `json:"mime_type"`
	UploadedAt       time.Time `json:"uploaded_at"`
}

// Transaction represents a buy/sell/split/grouping/bonus operation.
type Transaction struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	AssetID        string          `json:"asset_id"`
	DocumentID     *string         `json:"document_id,omitempty"`
	OperationType  string          `json:"operation_type"`
	Quantity       decimal.Decimal `json:"quantity"`
	UnitPrice      decimal.Decimal `json:"unit_price"`
	Costs          decimal.Decimal `json:"costs"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	OperationDate  time.Time       `json:"operation_date"`
	SettlementDate *time.Time      `json:"settlement_date,omitempty"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Earning represents a dividend, JCP, amortization or income event.
type Earning struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	AssetID     string          `json:"asset_id"`
	EarningType string          `json:"earning_type"`
	ComDate     time.Time       `json:"com_date"`
	PaymentDate time.Time       `json:"payment_date"`
	GrossAmount decimal.Decimal `json:"gross_amount"`
	TaxWithheld decimal.Decimal `json:"tax_withheld"`
	NetAmount   decimal.Decimal `json:"net_amount"`
	CreatedAt   time.Time       `json:"created_at"`
}

// MonthlyTaxBalance represents a monthly IR snapshot for a specific asset bucket.
type MonthlyTaxBalance struct {
	ID                     string          `json:"id"`
	UserID                 string          `json:"user_id"`
	YearMonth              string          `json:"year_month"`
	AssetBucket            string          `json:"asset_bucket"`
	TotalSales             decimal.Decimal `json:"total_sales"`
	GrossProfit            decimal.Decimal `json:"gross_profit"`
	LossesDeducted         decimal.Decimal `json:"losses_deducted"`
	AccumulatedLossCarried decimal.Decimal `json:"accumulated_loss_carried"`
	TaxableBase            decimal.Decimal `json:"taxable_base"`
	TaxRate                decimal.Decimal `json:"tax_rate"`
	TaxDue                 decimal.Decimal `json:"tax_due"`
	IRRFWithheld           decimal.Decimal `json:"irrf_withheld"`
	FinalDARF              decimal.Decimal `json:"final_darf"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

// MarketQuote represents a cached price quote for an asset ticker.
type MarketQuote struct {
	Ticker    string          `json:"ticker"`
	Price     decimal.Decimal `json:"price"`
	Currency  string          `json:"currency"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// MacroBenchmark represents a historical macro benchmark data point.
type MacroBenchmark struct {
	SeriesName    string          `json:"series_name"`
	ReferenceDate time.Time      `json:"reference_date"`
	RateValue     decimal.Decimal `json:"rate_value"`
}

// Asset class constants.
const (
	AssetClassStocks       = "STOCKS"
	AssetClassFII          = "FII"
	AssetClassFIAGRO       = "FIAGRO"
	AssetClassETF          = "ETF"
	AssetClassFixedIncome  = "FIXED_INCOME"
	AssetClassCrypto       = "CRYPTO"
	AssetClassInternational = "INTERNATIONAL"
)

// Operation type constants.
const (
	OpBuy      = "BUY"
	OpSell     = "SELL"
	OpSplit    = "SPLIT"
	OpGrouping = "GROUPING"
	OpBonus    = "BONUS"
)

// Earning type constants.
const (
	EarningDividend     = "DIVIDEND"
	EarningJCP          = "JCP"
	EarningAmortization = "AMORTIZATION"
	EarningIncome       = "INCOME"
)

// Tax bucket constants.
const (
	BucketStocksSwing    = "STOCKS_SWING"
	BucketStocksDaytrade = "STOCKS_DAYTRADE"
	BucketFII            = "FII"
	BucketETF            = "ETF"
	BucketCrypto         = "CRYPTO"
	BucketInternational  = "INTERNATIONAL"
)
