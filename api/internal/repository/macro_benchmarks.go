package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// MacroBenchmarkRepo handles persistence of macro benchmark data.
type MacroBenchmarkRepo struct {
	pool *pgxpool.Pool
}

// NewMacroBenchmarkRepo creates a new MacroBenchmarkRepo.
func NewMacroBenchmarkRepo(pool *pgxpool.Pool) *MacroBenchmarkRepo {
	return &MacroBenchmarkRepo{pool: pool}
}

// UpsertMany inserts or updates multiple macro benchmark data points in a batch.
func (r *MacroBenchmarkRepo) UpsertMany(ctx context.Context, benchmarks []domain.MacroBenchmark) error {
	if len(benchmarks) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, b := range benchmarks {
		_, err := tx.Exec(ctx, `
			INSERT INTO macro_benchmarks (series_name, reference_date, rate_value)
			VALUES ($1, $2, $3)
			ON CONFLICT (series_name, reference_date) DO UPDATE SET
				rate_value = EXCLUDED.rate_value
		`, b.SeriesName, b.ReferenceDate, b.RateValue)
		if err != nil {
			return fmt.Errorf("upserting benchmark %s/%v: %w", b.SeriesName, b.ReferenceDate, err)
		}
	}

	return tx.Commit(ctx)
}

// GetBySeriesAndRange retrieves benchmark data for a given series within a date range.
func (r *MacroBenchmarkRepo) GetBySeriesAndRange(ctx context.Context, seriesName string, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT series_name, reference_date, rate_value
		FROM macro_benchmarks
		WHERE series_name = $1
		  AND reference_date >= $2
		  AND reference_date <= $3
		ORDER BY reference_date ASC
	`, seriesName, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("querying benchmarks for %s: %w", seriesName, err)
	}
	defer rows.Close()

	var benchmarks []domain.MacroBenchmark
	for rows.Next() {
		var b domain.MacroBenchmark
		if err := rows.Scan(&b.SeriesName, &b.ReferenceDate, &b.RateValue); err != nil {
			return nil, fmt.Errorf("scanning benchmark: %w", err)
		}
		benchmarks = append(benchmarks, b)
	}

	return benchmarks, rows.Err()
}

// GetAllSeries retrieves all available data for all series within a date range.
func (r *MacroBenchmarkRepo) GetAllSeries(ctx context.Context, startDate, endDate time.Time) ([]domain.MacroBenchmark, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT series_name, reference_date, rate_value
		FROM macro_benchmarks
		WHERE reference_date >= $1
		  AND reference_date <= $2
		ORDER BY series_name, reference_date ASC
	`, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("querying all benchmarks: %w", err)
	}
	defer rows.Close()

	var benchmarks []domain.MacroBenchmark
	for rows.Next() {
		var b domain.MacroBenchmark
		if err := rows.Scan(&b.SeriesName, &b.ReferenceDate, &b.RateValue); err != nil {
			return nil, fmt.Errorf("scanning benchmark: %w", err)
		}
		benchmarks = append(benchmarks, b)
	}

	return benchmarks, rows.Err()
}

// GetLatestDate returns the most recent reference date for a given series.
// Used to determine what data needs to be fetched incrementally.
func (r *MacroBenchmarkRepo) GetLatestDate(ctx context.Context, seriesName string) (time.Time, error) {
	var latestDate time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(reference_date), '2000-01-01'::date)
		FROM macro_benchmarks
		WHERE series_name = $1
	`, seriesName).Scan(&latestDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("getting latest date for %s: %w", seriesName, err)
	}
	return latestDate, nil
}
