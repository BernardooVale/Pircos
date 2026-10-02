package handler

import (
	"net/http"

	"github.com/pircos/api/internal/repository"
	"github.com/pircos/api/internal/service"
)

// PortfolioHandler handles portfolio summary and history endpoints.
type PortfolioHandler struct {
	portfolioService *service.PortfolioService
	userRepo         *repository.UserRepo
}

// NewPortfolioHandler creates a new PortfolioHandler.
func NewPortfolioHandler(portfolioService *service.PortfolioService, userRepo *repository.UserRepo) *PortfolioHandler {
	return &PortfolioHandler{
		portfolioService: portfolioService,
		userRepo:         userRepo,
	}
}

// Summary handles GET /api/v1/portfolio/summary
func (h *PortfolioHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	summary, err := h.portfolioService.GetSummary(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed computing portfolio summary: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// History handles GET /api/v1/portfolio/history
func (h *PortfolioHandler) History(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	history, err := h.portfolioService.GetHistory(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed computing portfolio history: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, history)
}
