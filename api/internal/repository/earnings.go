package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// EarningRepo handles persistence of earnings (dividends, JCP, amortizations) in PostgreSQL.
type EarningRepo struct {
	pool *pgxpool.Pool
}

// NewEarningRepo creates a new EarningRepo instance.
func NewEarningRepo(pool *pgxpool.Pool) *EarningRepo {
	return &EarningRepo{pool: pool}
}

// Create inserts a single earning record.
func (r *EarningRepo) Create(ctx context.Context, e *domain.Earning) error {
	var id string
	var createdAt time.Time

	err := r.pool.QueryRow(ctx, `
		INSERT INTO earnings (
			user_id, asset_id, earning_type, com_date, payment_date,
			gross_amount, tax_withheld, net_amount
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`,
		e.UserID, e.AssetID, e.EarningType, e.ComDate, e.PaymentDate,
		e.GrossAmount, e.TaxWithheld, e.NetAmount,
	).Scan(&id, &createdAt)

	if err != nil {
		return fmt.Errorf("inserting earning: %w", err)
	}

	e.ID = id
	e.CreatedAt = createdAt
	return nil
}

// GetByUserID retrieves all earnings for a user, newest first.
func (r *EarningRepo) GetByUserID(ctx context.Context, userID string) ([]domain.Earning, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, user_id, asset_id, earning_type, com_date, payment_date,
			gross_amount, tax_withheld, net_amount, created_at
		FROM earnings
		WHERE user_id = $1
		ORDER BY payment_date DESC, com_date DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying earnings for user %s: %w", userID, err)
	}
	defer rows.Close()

	var earnings []domain.Earning
	for rows.Next() {
		var e domain.Earning
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AssetID, &e.EarningType, &e.ComDate, &e.PaymentDate,
			&e.GrossAmount, &e.TaxWithheld, &e.NetAmount, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning earning: %w", err)
		}
		earnings = append(earnings, e)
	}

	return earnings, rows.Err()
}

// GetByAssetID retrieves all earnings for an asset.
func (r *EarningRepo) GetByAssetID(ctx context.Context, userID, assetID string) ([]domain.Earning, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, user_id, asset_id, earning_type, com_date, payment_date,
			gross_amount, tax_withheld, net_amount, created_at
		FROM earnings
		WHERE user_id = $1 AND asset_id = $2
		ORDER BY payment_date ASC
	`, userID, assetID)
	if err != nil {
		return nil, fmt.Errorf("querying earnings for asset %s: %w", assetID, err)
	}
	defer rows.Close()

	var earnings []domain.Earning
	for rows.Next() {
		var e domain.Earning
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AssetID, &e.EarningType, &e.ComDate, &e.PaymentDate,
			&e.GrossAmount, &e.TaxWithheld, &e.NetAmount, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning earning: %w", err)
		}
		earnings = append(earnings, e)
	}

	return earnings, rows.Err()
}
