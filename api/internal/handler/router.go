package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dependencies holds handlers and database pool for the router.
type Dependencies struct {
	Pool               *pgxpool.Pool
	DocumentHandler    *DocumentHandler
	AssetHandler       *AssetHandler
	TransactionHandler *TransactionHandler
	EarningHandler     *EarningHandler
	PortfolioHandler   *PortfolioHandler
	TaxHandler         *TaxHandler
	BenchmarkHandler   *BenchmarkHandler
}

// Router creates the HTTP router with all registered handlers.
func Router(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", healthHandler(deps.Pool))

	// API v1 root
	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "pircos-api",
			"version": "1.0.0",
		})
	})

	// Document endpoints (Phase 4)
	if deps.DocumentHandler != nil {
		mux.HandleFunc("POST /api/v1/documents/parse", deps.DocumentHandler.Parse)
		mux.HandleFunc("POST /api/v1/documents/confirm", deps.DocumentHandler.Confirm)
	}

	// Assets endpoints (Phase 5.1)
	if deps.AssetHandler != nil {
		mux.HandleFunc("GET /api/v1/assets", deps.AssetHandler.List)
		mux.HandleFunc("POST /api/v1/assets", deps.AssetHandler.Create)
	}

	// Transactions endpoints (Phase 5.1)
	if deps.TransactionHandler != nil {
		mux.HandleFunc("GET /api/v1/transactions", deps.TransactionHandler.List)
		mux.HandleFunc("POST /api/v1/transactions", deps.TransactionHandler.Create)
	}

	// Earnings endpoints (Phase 5.1)
	if deps.EarningHandler != nil {
		mux.HandleFunc("GET /api/v1/earnings", deps.EarningHandler.List)
		mux.HandleFunc("POST /api/v1/earnings", deps.EarningHandler.Create)
	}

	// Portfolio endpoints (Phase 5.2)
	if deps.PortfolioHandler != nil {
		mux.HandleFunc("GET /api/v1/portfolio/summary", deps.PortfolioHandler.Summary)
		mux.HandleFunc("GET /api/v1/portfolio/history", deps.PortfolioHandler.History)
	}

	// Tax reports & drilldown endpoints (Phase 5.2)
	if deps.TaxHandler != nil {
		mux.HandleFunc("GET /api/v1/tax/monthly", deps.TaxHandler.Monthly)
		mux.HandleFunc("GET /api/v1/tax/drilldown", deps.TaxHandler.Drilldown)
	}

	// Macro benchmarks endpoints (Phase 5.2)
	if deps.BenchmarkHandler != nil {
		mux.HandleFunc("GET /api/v1/benchmarks", deps.BenchmarkHandler.List)
	}

	// CORS middleware wrapper
	return corsMiddleware(mux)
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		dbStatus := "ok"

		if err := pool.Ping(r.Context()); err != nil {
			status = "degraded"
			dbStatus = "error: " + err.Error()
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   status,
			"database": dbStatus,
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
