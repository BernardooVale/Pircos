package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// TransactionRepo handles transaction persistence in PostgreSQL.
type TransactionRepo struct {
	pool *pgxpool.Pool
}

// NewTransactionRepo creates a new TransactionRepo instance.
func NewTransactionRepo(pool *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{pool: pool}
}

// Create inserts a single transaction.
func (r *TransactionRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	var id string
	var createdAt time.Time

	err := r.pool.QueryRow(ctx, `
		INSERT INTO transactions (
			user_id, asset_id, document_id, operation_type,
			quantity, unit_price, costs, total_amount,
			operation_date, settlement_date, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`,
		tx.UserID, tx.AssetID, tx.DocumentID, tx.OperationType,
		tx.Quantity, tx.UnitPrice, tx.Costs, tx.TotalAmount,
		tx.OperationDate, tx.SettlementDate, tx.Metadata,
	).Scan(&id, &createdAt)

	if err != nil {
		return fmt.Errorf("inserting transaction: %w", err)
	}

	tx.ID = id
	tx.CreatedAt = createdAt
	return nil
}

// CreateMany inserts multiple transactions inside a single database transaction.
func (r *TransactionRepo) CreateMany(ctx context.Context, txs []domain.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	for i := range txs {
		var id string
		var createdAt time.Time
		err := dbTx.QueryRow(ctx, `
			INSERT INTO transactions (
				user_id, asset_id, document_id, operation_type,
				quantity, unit_price, costs, total_amount,
				operation_date, settlement_date, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id, created_at
		`,
			txs[i].UserID, txs[i].AssetID, txs[i].DocumentID, txs[i].OperationType,
			txs[i].Quantity, txs[i].UnitPrice, txs[i].Costs, txs[i].TotalAmount,
			txs[i].OperationDate, txs[i].SettlementDate, txs[i].Metadata,
		).Scan(&id, &createdAt)

		if err != nil {
			return fmt.Errorf("inserting transaction %d: %w", i, err)
		}
		txs[i].ID = id
		txs[i].CreatedAt = createdAt
	}

	return dbTx.Commit(ctx)
}

// GetByUserID returns all transactions for a user ordered chronologically by operation date.
func (r *TransactionRepo) GetByUserID(ctx context.Context, userID string) ([]domain.Transaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, user_id, asset_id, document_id, operation_type,
			quantity, unit_price, costs, total_amount,
			operation_date, settlement_date, metadata, created_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY operation_date ASC, created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying transactions for user %s: %w", userID, err)
	}
	defer rows.Close()

	var txs []domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(
			&tx.ID, &tx.UserID, &tx.AssetID, &tx.DocumentID, &tx.OperationType,
			&tx.Quantity, &tx.UnitPrice, &tx.Costs, &tx.TotalAmount,
			&tx.OperationDate, &tx.SettlementDate, &tx.Metadata, &tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, rows.Err()
}

// TransactionFilter defines query filters for listing transactions.
type TransactionFilter struct {
	UserID        string
	AssetID       string
	OperationType string
	StartDate     *time.Time
	EndDate       *time.Time
}

// GetFiltered retrieves transactions matching filter criteria.
func (r *TransactionRepo) GetFiltered(ctx context.Context, f TransactionFilter) ([]domain.Transaction, error) {
	query := `
		SELECT
			id, user_id, asset_id, document_id, operation_type,
			quantity, unit_price, costs, total_amount,
			operation_date, settlement_date, metadata, created_at
		FROM transactions
		WHERE user_id = $1
	`
	args := []any{f.UserID}
	argIdx := 2

	if f.AssetID != "" {
		query += fmt.Sprintf(" AND asset_id = $%d", argIdx)
		args = append(args, f.AssetID)
		argIdx++
	}
	if f.OperationType != "" {
		query += fmt.Sprintf(" AND operation_type = $%d", argIdx)
		args = append(args, f.OperationType)
		argIdx++
	}
	if f.StartDate != nil {
		query += fmt.Sprintf(" AND operation_date >= $%d", argIdx)
		args = append(args, *f.StartDate)
		argIdx++
	}
	if f.EndDate != nil {
		query += fmt.Sprintf(" AND operation_date <= $%d", argIdx)
		args = append(args, *f.EndDate)
		argIdx++
	}

	query += " ORDER BY operation_date DESC, created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying filtered transactions: %w", err)
	}
	defer rows.Close()

	var txs []domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(
			&tx.ID, &tx.UserID, &tx.AssetID, &tx.DocumentID, &tx.OperationType,
			&tx.Quantity, &tx.UnitPrice, &tx.Costs, &tx.TotalAmount,
			&tx.OperationDate, &tx.SettlementDate, &tx.Metadata, &tx.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning filtered transaction: %w", err)
		}
		txs = append(txs, tx)
	}

	return txs, rows.Err()
}
