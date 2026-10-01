package handler

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Router creates the HTTP router with all registered handlers.
func Router(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", healthHandler(pool))

	// API v1 routes (will be populated in subsequent phases)
	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "pircos-api",
			"version": "1.0.0",
		})
	})

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
