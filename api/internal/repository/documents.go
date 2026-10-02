package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pircos/api/internal/domain"
)

// DocumentRepo handles persistence of document metadata in PostgreSQL.
type DocumentRepo struct {
	pool *pgxpool.Pool
}

// NewDocumentRepo creates a new DocumentRepo instance.
func NewDocumentRepo(pool *pgxpool.Pool) *DocumentRepo {
	return &DocumentRepo{pool: pool}
}

// Create inserts a new document record.
func (r *DocumentRepo) Create(ctx context.Context, doc *domain.Document) error {
	var id string
	var uploadedAt time.Time

	err := r.pool.QueryRow(ctx, `
		INSERT INTO documents (
			user_id, original_filename, storage_path, file_size_bytes, mime_type
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, uploaded_at
	`, doc.UserID, doc.OriginalFilename, doc.StoragePath, doc.FileSizeBytes, doc.MimeType).Scan(&id, &uploadedAt)

	if err != nil {
		return fmt.Errorf("inserting document: %w", err)
	}

	doc.ID = id
	doc.UploadedAt = uploadedAt
	return nil
}

// GetByID retrieves a document by its ID.
func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*domain.Document, error) {
	var doc domain.Document
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, original_filename, storage_path, file_size_bytes, mime_type, uploaded_at
		FROM documents
		WHERE id = $1
	`, id).Scan(
		&doc.ID, &doc.UserID, &doc.OriginalFilename, &doc.StoragePath,
		&doc.FileSizeBytes, &doc.MimeType, &doc.UploadedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting document by id %s: %w", id, err)
	}
	return &doc, nil
}

// GetByUserID retrieves all documents uploaded by a user, newest first.
func (r *DocumentRepo) GetByUserID(ctx context.Context, userID string) ([]domain.Document, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, original_filename, storage_path, file_size_bytes, mime_type, uploaded_at
		FROM documents
		WHERE user_id = $1
		ORDER BY uploaded_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying documents for user %s: %w", userID, err)
	}
	defer rows.Close()

	var docs []domain.Document
	for rows.Next() {
		var doc domain.Document
		if err := rows.Scan(
			&doc.ID, &doc.UserID, &doc.OriginalFilename, &doc.StoragePath,
			&doc.FileSizeBytes, &doc.MimeType, &doc.UploadedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		docs = append(docs, doc)
	}

	return docs, rows.Err()
}

// Delete removes a document record by ID.
func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting document %s: %w", id, err)
	}
	return nil
}
