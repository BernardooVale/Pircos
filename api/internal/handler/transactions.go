package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
	"github.com/pircos/api/internal/service"
)

// TransactionHandler handles transaction-related endpoints.
type TransactionHandler struct {
	txRepo        *repository.TransactionRepo
	assetRepo     *repository.AssetRepo
	userRepo      *repository.UserRepo
	cascadeEngine *service.CascadeEngine
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler(
	txRepo *repository.TransactionRepo,
	assetRepo *repository.AssetRepo,
	userRepo *repository.UserRepo,
	cascadeEngine *service.CascadeEngine,
) *TransactionHandler {
	return &TransactionHandler{
		txRepo:        txRepo,
		assetRepo:     assetRepo,
		userRepo:      userRepo,
		cascadeEngine: cascadeEngine,
	}
}

// EnrichedTransaction represents a transaction enriched with ticker and asset name.
type EnrichedTransaction struct {
	domain.Transaction
	Ticker     string `json:"ticker"`
	AssetName  string `json:"asset_name"`
	AssetClass string `json:"asset_class"`
}

// List handles GET /api/v1/transactions
func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	userID := q.Get("user_id")
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	filter := repository.TransactionFilter{
		UserID:        userID,
		AssetID:       q.Get("asset_id"),
		OperationType: q.Get("operation_type"),
	}

	if startStr := q.Get("start_date"); startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			filter.StartDate = &t
		}
	}
	if endStr := q.Get("end_date"); endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			filter.EndDate = &t
		}
	}

	txs, err := h.txRepo.GetFiltered(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed querying transactions: "+err.Error())
		return
	}

	// Enrich with asset data
	assetMap, _ := h.assetRepo.GetAllMap(ctx)
	var enriched []EnrichedTransaction
	for _, tx := range txs {
		et := EnrichedTransaction{Transaction: tx}
		if asset, ok := assetMap[tx.AssetID]; ok {
			et.Ticker = asset.Ticker
			et.AssetName = asset.Name
			et.AssetClass = asset.AssetClass
		}
		enriched = append(enriched, et)
	}

	if enriched == nil {
		enriched = []EnrichedTransaction{}
	}

	writeJSON(w, http.StatusOK, enriched)
}

// CreateTransactionRequest represents the body for manual transaction creation.
type CreateTransactionRequest struct {
	UserID         string          `json:"user_id,omitempty"`
	Ticker         string          `json:"ticker"`
	AssetID        string          `json:"asset_id,omitempty"`
	AssetName      string          `json:"asset_name,omitempty"`
	AssetClass     string          `json:"asset_class,omitempty"`
	OperationType  string          `json:"operation_type"` // BUY, SELL, SPLIT, GROUPING, BONUS
	Quantity       decimal.Decimal `json:"quantity"`
	UnitPrice      decimal.Decimal `json:"unit_price"`
	Costs          decimal.Decimal `json:"costs"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	OperationDate  string          `json:"operation_date"` // YYYY-MM-DD or RFC3339
	SettlementDate *string         `json:"settlement_date,omitempty"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
}

// Create handles POST /api/v1/transactions
func (h *TransactionHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	if req.OperationType == "" || req.Quantity.IsZero() {
		writeError(w, http.StatusBadRequest, "Operation type and non-zero quantity are required")
		return
	}

	userID := req.UserID
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	// Resolve asset
	assetID := req.AssetID
	if assetID == "" && req.Ticker != "" {
		name := req.AssetName
		if name == "" {
			name = req.Ticker
		}
		class := req.AssetClass
		if class == "" {
			class = domain.AssetClassStocks
		}
		asset, err := h.assetRepo.GetOrCreate(ctx, req.Ticker, name, class, "BRL")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving asset: "+err.Error())
			return
		}
		assetID = asset.ID
	}

	if assetID == "" {
		writeError(w, http.StatusBadRequest, "Either ticker or asset_id is required")
		return
	}

	// Parse date
	opDate, err := time.Parse(time.RFC3339, req.OperationDate)
	if err != nil {
		opDate, err = time.Parse("2006-01-02", req.OperationDate)
		if err != nil {
			opDate = time.Now()
		}
	}

	var setDate *time.Time
	if req.SettlementDate != nil && *req.SettlementDate != "" {
		if sd, err := time.Parse("2006-01-02", *req.SettlementDate); err == nil {
			setDate = &sd
		} else if sd, err := time.Parse(time.RFC3339, *req.SettlementDate); err == nil {
			setDate = &sd
		}
	}

	// Calculate total amount if not provided
	totalAmount := req.TotalAmount
	if totalAmount.IsZero() {
		totalAmount = req.Quantity.Mul(req.UnitPrice)
	}

	tx := &domain.Transaction{
		UserID:         userID,
		AssetID:        assetID,
		OperationType:  req.OperationType,
		Quantity:       req.Quantity,
		UnitPrice:      req.UnitPrice,
		Costs:          req.Costs,
		TotalAmount:    totalAmount,
		OperationDate:  opDate,
		SettlementDate: setDate,
		Metadata:       req.Metadata,
	}

	if err := h.txRepo.Create(ctx, tx); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed creating transaction: "+err.Error())
		return
	}

	// Trigger cascade recalculation
	if h.cascadeEngine != nil {
		if err := h.cascadeEngine.Recalculate(ctx, userID); err != nil {
			// Log warning
			println("[TRANSACTION] Warning: cascade recalculation failed: " + err.Error())
		}
	}

	writeJSON(w, http.StatusCreated, tx)
}
