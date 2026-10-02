package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pircos/api/internal/config"
	"github.com/pircos/api/internal/handler"
	"github.com/pircos/api/internal/repository"
	"github.com/pircos/api/internal/service"
	"github.com/pircos/api/internal/storage"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to database
	pool, err := repository.NewPool(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("[MAIN] Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	migrationsDir := getMigrationsDir()
	if err := repository.RunMigrations(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("[MAIN] Failed to run migrations: %v", err)
	}

	// Storage
	fileStore, err := storage.NewLocalStorage(cfg.StorageDir)
	if err != nil {
		log.Fatalf("[MAIN] Failed to initialize storage: %v", err)
	}

	// Repositories
	docRepo := repository.NewDocumentRepo(pool)
	assetRepo := repository.NewAssetRepo(pool)
	txRepo := repository.NewTransactionRepo(pool)
	earningRepo := repository.NewEarningRepo(pool)
	userRepo := repository.NewUserRepo(pool)
	balanceRepo := repository.NewMonthlyTaxBalanceRepo(pool)
	quoteRepo := repository.NewMarketQuoteRepo(pool)
	benchmarkRepo := repository.NewMacroBenchmarkRepo(pool)

	// Services
	sinacorParser := service.NewSinacorParser()
	taxEngine := service.NewTaxEngine()
	cascadeEngine := service.NewCascadeEngine(taxEngine, balanceRepo, txRepo, assetRepo, earningRepo)
	marketData := service.NewMarketDataService(quoteRepo, benchmarkRepo)
	portfolioService := service.NewPortfolioService(txRepo, assetRepo, earningRepo, marketData, benchmarkRepo)

	// Handlers
	docHandler := handler.NewDocumentHandler(
		sinacorParser, fileStore, docRepo, assetRepo, txRepo, userRepo, cascadeEngine,
	)
	assetHandler := handler.NewAssetHandler(assetRepo)
	txHandler := handler.NewTransactionHandler(txRepo, assetRepo, userRepo, cascadeEngine)
	earningHandler := handler.NewEarningHandler(earningRepo, assetRepo, userRepo, cascadeEngine)
	portfolioHandler := handler.NewPortfolioHandler(portfolioService, userRepo)
	taxHandler := handler.NewTaxHandler(balanceRepo, txRepo, assetRepo, userRepo)
	benchmarkHandler := handler.NewBenchmarkHandler(marketData)

	// Setup HTTP server
	router := handler.Router(handler.Dependencies{
		Pool:               pool,
		DocumentHandler:    docHandler,
		AssetHandler:       assetHandler,
		TransactionHandler: txHandler,
		EarningHandler:     earningHandler,
		PortfolioHandler:   portfolioHandler,
		TaxHandler:         taxHandler,
		BenchmarkHandler:   benchmarkHandler,
	})
	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("[MAIN] Shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("[MAIN] Server shutdown error: %v", err)
		}
		cancel()
	}()

	log.Printf("[MAIN] Pircos API starting on %s", cfg.Addr())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[MAIN] Server error: %v", err)
	}
	log.Println("[MAIN] Server stopped")
}

// getMigrationsDir resolves the migrations directory path.
// In Docker, migrations are at /app/migrations.
// In local dev, they're relative to the working directory.
func getMigrationsDir() string {
	// Check if running inside Docker (migrations copied by Dockerfile)
	dockerPath := "/app/migrations"
	if _, err := os.Stat(dockerPath); err == nil {
		return dockerPath
	}

	// Local development: relative to working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("[MAIN] Cannot determine working directory: %v", err)
	}

	// Try ./migrations first (if run from api/)
	localPath := filepath.Join(wd, "migrations")
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// Try api/migrations (if run from project root)
	rootPath := filepath.Join(wd, "api", "migrations")
	if _, err := os.Stat(rootPath); err == nil {
		return rootPath
	}

	log.Fatalf("[MAIN] Cannot find migrations directory (tried %s and %s)", localPath, rootPath)
	return ""
}
