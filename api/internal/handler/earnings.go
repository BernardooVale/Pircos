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

// EarningHandler handles earning-related HTTP requests.
type EarningHandler struct {
	earningRepo   *repository.EarningRepo
	assetRepo     *repository.AssetRepo
	userRepo      *repository.UserRepo
	cascadeEngine *service.CascadeEngine
}

// NewEarningHandler creates a new EarningHandler.
func NewEarningHandler(
	earningRepo *repository.EarningRepo,
	assetRepo *repository.AssetRepo,
	userRepo *repository.UserRepo,
	cascadeEngine *service.CascadeEngine,
) *EarningHandler {
	return &EarningHandler{
		earningRepo:   earningRepo,
		assetRepo:     assetRepo,
		userRepo:      userRepo,
		cascadeEngine: cascadeEngine,
	}
}

// EnrichedEarning represents an earning record enriched with ticker and asset name.
type EnrichedEarning struct {
	domain.Earning
	Ticker    string `json:"ticker"`
	AssetName string `json:"asset_name"`
}

// List handles GET /api/v1/earnings
func (h *EarningHandler) List(w http.ResponseWriter, r *http.Request) {
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

	assetID := q.Get("asset_id")

	var earnings []domain.Earning
	var err error
	if assetID != "" {
		earnings, err = h.earningRepo.GetByAssetID(ctx, userID, assetID)
	} else {
		earnings, err = h.earningRepo.GetByUserID(ctx, userID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed querying earnings: "+err.Error())
		return
	}

	// Enrich with asset data
	assetMap, _ := h.assetRepo.GetAllMap(ctx)
	var enriched []EnrichedEarning
	for _, earn := range earnings {
		ee := EnrichedEarning{Earning: earn}
		if a, ok := assetMap[earn.AssetID]; ok {
			ee.Ticker = a.Ticker
			ee.AssetName = a.Name
		}
		enriched = append(enriched, ee)
	}

	if enriched == nil {
		enriched = []EnrichedEarning{}
	}

	writeJSON(w, http.StatusOK, enriched)
}

// CreateEarningRequest represents the payload for registering an earning.
type CreateEarningRequest struct {
	UserID      string          `json:"user_id,omitempty"`
	Ticker      string          `json:"ticker"`
	AssetID     string          `json:"asset_id,omitempty"`
	EarningType string          `json:"earning_type"` // DIVIDEND, JCP, AMORTIZATION, INCOME
	ComDate     string          `json:"com_date"`     // YYYY-MM-DD
	PaymentDate string          `json:"payment_date"` // YYYY-MM-DD
	GrossAmount decimal.Decimal `json:"gross_amount"`
	TaxWithheld decimal.Decimal `json:"tax_withheld"`
	NetAmount   decimal.Decimal `json:"net_amount"`
}

// Create handles POST /api/v1/earnings
func (h *EarningHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateEarningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	if req.EarningType == "" || req.GrossAmount.IsZero() {
		writeError(w, http.StatusBadRequest, "Earning type and gross amount are required")
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
		asset, err := h.assetRepo.GetOrCreate(ctx, req.Ticker, req.Ticker, domain.AssetClassStocks, "BRL")
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

	comDate, err := time.Parse("2006-01-02", req.ComDate)
	if err != nil {
		comDate = time.Now()
	}

	paymentDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		paymentDate = comDate
	}

	netAmount := req.NetAmount
	if netAmount.IsZero() {
		netAmount = req.GrossAmount.Sub(req.TaxWithheld)
	}

	earning := &domain.Earning{
		UserID:      userID,
		AssetID:     assetID,
		EarningType: req.EarningType,
		ComDate:     comDate,
		PaymentDate: paymentDate,
		GrossAmount: req.GrossAmount,
		TaxWithheld: req.TaxWithheld,
		NetAmount:   netAmount,
	}

	if err := h.earningRepo.Create(ctx, earning); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed creating earning: "+err.Error())
		return
	}

	// Amortization affects FII cost basis -> trigger cascade recalculation
	if req.EarningType == domain.EarningAmortization && h.cascadeEngine != nil {
		_ = h.cascadeEngine.Recalculate(ctx, userID)
	}

	writeJSON(w, http.StatusCreated, earning)
}
