package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// AssetRepo handles asset data persistence.
type AssetRepo struct {
	pool *pgxpool.Pool
}

// NewAssetRepo creates a new AssetRepo instance.
func NewAssetRepo(pool *pgxpool.Pool) *AssetRepo {
	return &AssetRepo{pool: pool}
}

// GetOrCreate retrieves an asset by ticker or inserts it if it does not exist.
func (r *AssetRepo) GetOrCreate(ctx context.Context, ticker, name, assetClass, currency string) (*domain.Asset, error) {
	if currency == "" {
		currency = "BRL"
	}
	if name == "" {
		name = ticker
	}
	if assetClass == "" {
		assetClass = domain.AssetClassStocks
	}

	var asset domain.Asset
	err := r.pool.QueryRow(ctx, `
		INSERT INTO assets (ticker, name, asset_class, currency)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ticker) DO UPDATE SET
			name = CASE WHEN assets.name = assets.ticker THEN EXCLUDED.name ELSE assets.name END
		RETURNING id, ticker, name, asset_class, currency, created_at
	`, ticker, name, assetClass, currency).Scan(
		&asset.ID, &asset.Ticker, &asset.Name, &asset.AssetClass, &asset.Currency, &asset.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get or create asset %s: %w", ticker, err)
	}

	return &asset, nil
}

// GetByTicker retrieves an asset by its ticker symbol.
func (r *AssetRepo) GetByTicker(ctx context.Context, ticker string) (*domain.Asset, error) {
	var a domain.Asset
	err := r.pool.QueryRow(ctx, `
		SELECT id, ticker, name, asset_class, currency, created_at
		FROM assets
		WHERE ticker = $1
	`, ticker).Scan(&a.ID, &a.Ticker, &a.Name, &a.AssetClass, &a.Currency, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting asset by ticker %s: %w", ticker, err)
	}
	return &a, nil
}

// GetAll returns all registered assets.
func (r *AssetRepo) GetAll(ctx context.Context) ([]domain.Asset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ticker, name, asset_class, currency, created_at
		FROM assets
		ORDER BY ticker ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying assets: %w", err)
	}
	defer rows.Close()

	var assets []domain.Asset
	for rows.Next() {
		var a domain.Asset
		if err := rows.Scan(&a.ID, &a.Ticker, &a.Name, &a.AssetClass, &a.Currency, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning asset: %w", err)
		}
		assets = append(assets, a)
	}

	return assets, rows.Err()
}

// GetAllMap returns all assets keyed by ID.
func (r *AssetRepo) GetAllMap(ctx context.Context) (map[string]domain.Asset, error) {
	assets, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]domain.Asset, len(assets))
	for _, a := range assets {
		m[a.ID] = a
	}
	return m, nil
}
