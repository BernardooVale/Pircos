package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/pircos/api/internal/domain"
	"github.com/pircos/api/internal/repository"
	"github.com/pircos/api/internal/service"
	"github.com/pircos/api/internal/storage"
)

// DocumentHandler handles document upload, parsing, and confirmation endpoints.
type DocumentHandler struct {
	parser        *service.SinacorParser
	storage       storage.FileStorage
	docRepo       *repository.DocumentRepo
	assetRepo     *repository.AssetRepo
	txRepo        *repository.TransactionRepo
	userRepo      *repository.UserRepo
	cascadeEngine *service.CascadeEngine
}

// NewDocumentHandler creates a new DocumentHandler instance.
func NewDocumentHandler(
	parser *service.SinacorParser,
	storage storage.FileStorage,
	docRepo *repository.DocumentRepo,
	assetRepo *repository.AssetRepo,
	txRepo *repository.TransactionRepo,
	userRepo *repository.UserRepo,
	cascadeEngine *service.CascadeEngine,
) *DocumentHandler {
	return &DocumentHandler{
		parser:        parser,
		storage:       storage,
		docRepo:       docRepo,
		assetRepo:     assetRepo,
		txRepo:        txRepo,
		userRepo:      userRepo,
		cascadeEngine: cascadeEngine,
	}
}

// Parse handles POST /api/v1/documents/parse
// Receives a PDF file via multipart/form-data, extracts operations and costs,
// and returns the structured data for frontend review without persisting.
func (h *DocumentHandler) Parse(w http.ResponseWriter, r *http.Request) {
	// 32MB max in memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing 'file' field in multipart form")
		return
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading file: "+err.Error())
		return
	}

	note, err := h.parser.ParsePDF(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		// Try fallback to text parsing if PDF structure extraction failed
		note, err = h.parser.ParseText(string(buf))
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "Failed parsing document: "+err.Error())
			return
		}
	}

	if note.BrokerName == "" {
		note.BrokerName = header.Filename
	}

	writeJSON(w, http.StatusOK, note)
}

// ConfirmPayload represents the request body for POST /api/v1/documents/confirm
type ConfirmPayload struct {
	UserID         string                   `json:"user_id,omitempty"`
	NoteNumber     string                   `json:"note_number"`
	BrokerName     string                   `json:"broker_name"`
	OperationDate  string                   `json:"operation_date"`
	SettlementDate *string                  `json:"settlement_date,omitempty"`
	Transactions   []ConfirmTransactionItem `json:"transactions"`
}

// ConfirmTransactionItem represents a confirmed transaction to persist.
type ConfirmTransactionItem struct {
	OperationType    string          `json:"operation_type"` // "BUY" or "SELL"
	Ticker           string          `json:"ticker"`
	AssetName        string          `json:"asset_name,omitempty"`
	AssetClass       string          `json:"asset_class"`
	Quantity         decimal.Decimal `json:"quantity"`
	UnitPrice        decimal.Decimal `json:"unit_price"`
	Costs            decimal.Decimal `json:"costs"`
	TotalAmount      decimal.Decimal `json:"total_amount"`
	IsDayTrade       bool            `json:"is_day_trade"`
}

// Confirm handles POST /api/v1/documents/confirm
// Saves the document file, creates asset records, persists transactions,
// and triggers the cascade recalculation of tax balances.
func (h *DocumentHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload ConfirmPayload
	var fileBytes []byte
	var filename string

	// Check if request is multipart/form-data or JSON
	contentType := r.Header.Get("Content-Type")
	if bytes.HasPrefix([]byte(contentType), []byte("multipart/form-data")) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid multipart form: "+err.Error())
			return
		}

		dataStr := r.FormValue("data")
		if dataStr == "" {
			writeError(w, http.StatusBadRequest, "Missing 'data' JSON field in form")
			return
		}
		if err := json.Unmarshal([]byte(dataStr), &payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON in 'data' field: "+err.Error())
			return
		}

		file, header, err := r.FormFile("file")
		if err == nil && file != nil {
			defer file.Close()
			fileBytes, _ = io.ReadAll(file)
			filename = header.Filename
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
			return
		}
	}

	// 1. Resolve user ID
	userID := payload.UserID
	if userID == "" {
		defaultUser, err := h.userRepo.GetOrCreateDefault(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed resolving user: "+err.Error())
			return
		}
		userID = defaultUser.ID
	}

	// 2. Save file if provided
	var docID *string
	if len(fileBytes) > 0 {
		if filename == "" {
			filename = fmt.Sprintf("nota_%s.pdf", payload.NoteNumber)
		}
		storagePath, size, err := h.storage.Save(ctx, filename, bytes.NewReader(fileBytes))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed saving document to storage: "+err.Error())
			return
		}

		doc := &domain.Document{
			UserID:           userID,
			OriginalFilename: filename,
			StoragePath:      storagePath,
			FileSizeBytes:    size,
			MimeType:         "application/pdf",
		}
		if err := h.docRepo.Create(ctx, doc); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed recording document: "+err.Error())
			return
		}
		docID = &doc.ID
	}

	// Parse operation date
	opDate, err := time.Parse(time.RFC3339, payload.OperationDate)
	if err != nil {
		opDate, err = time.Parse("2006-01-02", payload.OperationDate)
		if err != nil {
			opDate = time.Now()
		}
	}

	var setDate *time.Time
	if payload.SettlementDate != nil && *payload.SettlementDate != "" {
		if sd, err := time.Parse("2006-01-02", *payload.SettlementDate); err == nil {
			setDate = &sd
		} else if sd, err := time.Parse(time.RFC3339, *payload.SettlementDate); err == nil {
			setDate = &sd
		}
	}

	// 3. Process each confirmed transaction
	var txsToInsert []domain.Transaction
	for _, item := range payload.Transactions {
		assetName := item.AssetName
		if assetName == "" {
			assetName = item.Ticker
		}
		assetClass := item.AssetClass
		if assetClass == "" {
			assetClass = domain.AssetClassStocks
		}

		asset, err := h.assetRepo.GetOrCreate(ctx, item.Ticker, assetName, assetClass, "BRL")
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed registering asset %s: %v", item.Ticker, err))
			return
		}

		metadata := map[string]any{
			"note_number":  payload.NoteNumber,
			"broker_name":  payload.BrokerName,
			"is_day_trade": item.IsDayTrade,
		}

		txsToInsert = append(txsToInsert, domain.Transaction{
			UserID:         userID,
			AssetID:        asset.ID,
			DocumentID:     docID,
			OperationType:  item.OperationType,
			Quantity:       item.Quantity,
			UnitPrice:      item.UnitPrice,
			Costs:          item.Costs,
			TotalAmount:    item.TotalAmount,
			OperationDate:  opDate,
			SettlementDate: setDate,
			Metadata:       metadata,
		})
	}

	// 4. Persist transactions
	if err := h.txRepo.CreateMany(ctx, txsToInsert); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed persisting transactions: "+err.Error())
		return
	}

	// 5. Trigger cascade recalculation
	if h.cascadeEngine != nil {
		if err := h.cascadeEngine.Recalculate(ctx, userID); err != nil {
			// Log error but don't fail confirmation since transactions were already saved
			fmt.Printf("[CONFIRM] Warning: cascade recalculation failed: %v\n", err)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":             "confirmed",
		"document_id":        docID,
		"transactions_saved": len(txsToInsert),
		"note_number":        payload.NoteNumber,
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{
		"error": msg,
	})
}
