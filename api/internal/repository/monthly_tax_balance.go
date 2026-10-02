package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// MonthlyTaxBalanceRepo handles persistence of monthly tax snapshots.
type MonthlyTaxBalanceRepo struct {
	pool *pgxpool.Pool
}

// NewMonthlyTaxBalanceRepo creates a new MonthlyTaxBalanceRepo.
func NewMonthlyTaxBalanceRepo(pool *pgxpool.Pool) *MonthlyTaxBalanceRepo {
	return &MonthlyTaxBalanceRepo{pool: pool}
}

// UpsertMany inserts or updates multiple monthly tax balance records.
func (r *MonthlyTaxBalanceRepo) UpsertMany(ctx context.Context, balances []domain.MonthlyTaxBalance) error {
	if len(balances) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, b := range balances {
		_, err := tx.Exec(ctx, `
			INSERT INTO monthly_tax_balance (
				user_id, year_month, asset_bucket, total_sales, gross_profit,
				losses_deducted, accumulated_loss_carried, taxable_base,
				tax_rate, tax_due, irrf_withheld, final_darf, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (user_id, year_month, asset_bucket) DO UPDATE SET
				total_sales = EXCLUDED.total_sales,
				gross_profit = EXCLUDED.gross_profit,
				losses_deducted = EXCLUDED.losses_deducted,
				accumulated_loss_carried = EXCLUDED.accumulated_loss_carried,
				taxable_base = EXCLUDED.taxable_base,
				tax_rate = EXCLUDED.tax_rate,
				tax_due = EXCLUDED.tax_due,
				irrf_withheld = EXCLUDED.irrf_withheld,
				final_darf = EXCLUDED.final_darf,
				updated_at = EXCLUDED.updated_at
		`,
			b.UserID, b.YearMonth, b.AssetBucket, b.TotalSales, b.GrossProfit,
			b.LossesDeducted, b.AccumulatedLossCarried, b.TaxableBase,
			b.TaxRate, b.TaxDue, b.IRRFWithheld, b.FinalDARF, time.Now(),
		)
		if err != nil {
			return fmt.Errorf("upserting monthly tax balance for %s/%s/%s: %w",
				b.UserID, b.YearMonth, b.AssetBucket, err)
		}
	}

	return tx.Commit(ctx)
}

// GetByUserAndYear returns all monthly tax balance records for a user in a given year (e.g. "2024").
func (r *MonthlyTaxBalanceRepo) GetByUserAndYear(ctx context.Context, userID, year string) ([]domain.MonthlyTaxBalance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, user_id, year_month, asset_bucket, total_sales, gross_profit,
			losses_deducted, accumulated_loss_carried, taxable_base,
			tax_rate, tax_due, irrf_withheld, final_darf, updated_at
		FROM monthly_tax_balance
		WHERE user_id = $1 AND year_month LIKE $2 || '-%'
		ORDER BY year_month ASC, asset_bucket ASC
	`, userID, year)
	if err != nil {
		return nil, fmt.Errorf("querying monthly tax balance by year %s: %w", year, err)
	}
	defer rows.Close()

	var balances []domain.MonthlyTaxBalance
	for rows.Next() {
		var b domain.MonthlyTaxBalance
		if err := rows.Scan(
			&b.ID, &b.UserID, &b.YearMonth, &b.AssetBucket, &b.TotalSales, &b.GrossProfit,
			&b.LossesDeducted, &b.AccumulatedLossCarried, &b.TaxableBase,
			&b.TaxRate, &b.TaxDue, &b.IRRFWithheld, &b.FinalDARF, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning monthly tax balance: %w", err)
		}
		balances = append(balances, b)
	}

	return balances, rows.Err()
}

// GetByUserAndMonth returns all bucket balances for a user in a given month (YYYY-MM).
func (r *MonthlyTaxBalanceRepo) GetByUserAndMonth(ctx context.Context, userID, yearMonth string) ([]domain.MonthlyTaxBalance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, user_id, year_month, asset_bucket, total_sales, gross_profit,
			losses_deducted, accumulated_loss_carried, taxable_base,
			tax_rate, tax_due, irrf_withheld, final_darf, updated_at
		FROM monthly_tax_balance
		WHERE user_id = $1 AND year_month = $2
		ORDER BY asset_bucket ASC
	`, userID, yearMonth)
	if err != nil {
		return nil, fmt.Errorf("querying monthly tax balance for %s: %w", yearMonth, err)
	}
	defer rows.Close()

	var balances []domain.MonthlyTaxBalance
	for rows.Next() {
		var b domain.MonthlyTaxBalance
		if err := rows.Scan(
			&b.ID, &b.UserID, &b.YearMonth, &b.AssetBucket, &b.TotalSales, &b.GrossProfit,
			&b.LossesDeducted, &b.AccumulatedLossCarried, &b.TaxableBase,
			&b.TaxRate, &b.TaxDue, &b.IRRFWithheld, &b.FinalDARF, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning monthly tax balance: %w", err)
		}
		balances = append(balances, b)
	}

	return balances, rows.Err()
}

// GetByUserMonthBucket returns a specific bucket balance.
func (r *MonthlyTaxBalanceRepo) GetByUserMonthBucket(ctx context.Context, userID, yearMonth, bucket string) (*domain.MonthlyTaxBalance, error) {
	var b domain.MonthlyTaxBalance
	err := r.pool.QueryRow(ctx, `
		SELECT
			id, user_id, year_month, asset_bucket, total_sales, gross_profit,
			losses_deducted, accumulated_loss_carried, taxable_base,
			tax_rate, tax_due, irrf_withheld, final_darf, updated_at
		FROM monthly_tax_balance
		WHERE user_id = $1 AND year_month = $2 AND asset_bucket = $3
	`, userID, yearMonth, bucket).Scan(
		&b.ID, &b.UserID, &b.YearMonth, &b.AssetBucket, &b.TotalSales, &b.GrossProfit,
		&b.LossesDeducted, &b.AccumulatedLossCarried, &b.TaxableBase,
		&b.TaxRate, &b.TaxDue, &b.IRRFWithheld, &b.FinalDARF, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting monthly tax balance for %s/%s/%s: %w", userID, yearMonth, bucket, err)
	}
	return &b, nil
}

// DeleteFromMonth deletes all balances from a given yearMonth forward (inclusive) for a user.
func (r *MonthlyTaxBalanceRepo) DeleteFromMonth(ctx context.Context, userID, yearMonth string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM monthly_tax_balance
		WHERE user_id = $1 AND year_month >= $2
	`, userID, yearMonth)
	if err != nil {
		return fmt.Errorf("deleting monthly tax balances from %s: %w", yearMonth, err)
	}
	return nil
}
