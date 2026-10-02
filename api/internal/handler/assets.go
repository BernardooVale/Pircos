package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
)

// AssetHandler handles asset-related HTTP requests.
type AssetHandler struct {
	assetRepo *repository.AssetRepo
}

// NewAssetHandler creates a new AssetHandler.
func NewAssetHandler(assetRepo *repository.AssetRepo) *AssetHandler {
	return &AssetHandler{assetRepo: assetRepo}
}

// List handles GET /api/v1/assets
func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
	assets, err := h.assetRepo.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed querying assets: "+err.Error())
		return
	}
	if assets == nil {
		assets = []domain.Asset{}
	}
	writeJSON(w, http.StatusOK, assets)
}

// CreateAssetRequest represents payload for creating an asset.
type CreateAssetRequest struct {
	Ticker     string `json:"ticker"`
	Name       string `json:"name"`
	AssetClass string `json:"asset_class"`
	Currency   string `json:"currency"`
}

// Create handles POST /api/v1/assets
func (h *AssetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Ticker == "" {
		writeError(w, http.StatusBadRequest, "Ticker is required")
		return
	}

	asset, err := h.assetRepo.GetOrCreate(r.Context(), req.Ticker, req.Name, req.AssetClass, req.Currency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed creating asset: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, asset)
}
