package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
)

// MarketQuoteRepo handles persistence of market quote data.
type MarketQuoteRepo struct {
	pool *pgxpool.Pool
}

// NewMarketQuoteRepo creates a new MarketQuoteRepo.
func NewMarketQuoteRepo(pool *pgxpool.Pool) *MarketQuoteRepo {
	return &MarketQuoteRepo{pool: pool}
}

// Upsert inserts or updates a market quote for the given ticker.
func (r *MarketQuoteRepo) Upsert(ctx context.Context, quote *domain.MarketQuote) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO market_quotes (ticker, price, currency, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ticker) DO UPDATE SET
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			updated_at = EXCLUDED.updated_at
	`, quote.Ticker, quote.Price, quote.Currency, quote.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting market quote for %s: %w", quote.Ticker, err)
	}
	return nil
}

// UpsertMany inserts or updates multiple market quotes in a batch.
func (r *MarketQuoteRepo) UpsertMany(ctx context.Context, quotes []*domain.MarketQuote) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, q := range quotes {
		_, err := tx.Exec(ctx, `
			INSERT INTO market_quotes (ticker, price, currency, updated_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (ticker) DO UPDATE SET
				price = EXCLUDED.price,
				currency = EXCLUDED.currency,
				updated_at = EXCLUDED.updated_at
		`, q.Ticker, q.Price, q.Currency, q.UpdatedAt)
		if err != nil {
			return fmt.Errorf("upserting market quote for %s: %w", q.Ticker, err)
		}
	}

	return tx.Commit(ctx)
}

// GetByTicker retrieves a cached market quote for the given ticker.
func (r *MarketQuoteRepo) GetByTicker(ctx context.Context, ticker string) (*domain.MarketQuote, error) {
	var q domain.MarketQuote
	var price decimal.Decimal
	err := r.pool.QueryRow(ctx, `
		SELECT ticker, price, currency, updated_at
		FROM market_quotes
		WHERE ticker = $1
	`, ticker).Scan(&q.Ticker, &price, &q.Currency, &q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting market quote for %s: %w", ticker, err)
	}
	q.Price = price
	return &q, nil
}

// GetAll retrieves all cached market quotes.
func (r *MarketQuoteRepo) GetAll(ctx context.Context) ([]*domain.MarketQuote, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ticker, price, currency, updated_at
		FROM market_quotes
		ORDER BY ticker
	`)
	if err != nil {
		return nil, fmt.Errorf("listing market quotes: %w", err)
	}
	defer rows.Close()

	var quotes []*domain.MarketQuote
	for rows.Next() {
		var q domain.MarketQuote
		if err := rows.Scan(&q.Ticker, &q.Price, &q.Currency, &q.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning market quote: %w", err)
		}
		quotes = append(quotes, &q)
	}

	return quotes, rows.Err()
}

// GetFreshByTicker retrieves a cached quote only if it was updated within maxAge.
func (r *MarketQuoteRepo) GetFreshByTicker(ctx context.Context, ticker string, maxAge time.Duration) (*domain.MarketQuote, error) {
	q, err := r.GetByTicker(ctx, ticker)
	if err != nil {
		return nil, err
	}
	if time.Since(q.UpdatedAt) > maxAge {
		return nil, fmt.Errorf("cached quote for %s is stale (age: %v, max: %v)", ticker, time.Since(q.UpdatedAt), maxAge)
	}
	return q, nil
}
