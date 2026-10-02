package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssetHandler_Create_Validation(t *testing.T) {
	h := NewAssetHandler(nil)

	// Missing ticker
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", bytes.NewReader([]byte(`{"name": "Petrobras"}`)))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTransactionHandler_Create_Validation(t *testing.T) {
	h := NewTransactionHandler(nil, nil, nil, nil)

	// Missing quantity and operation type
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader([]byte(`{"ticker": "PETR4"}`)))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEarningHandler_Create_Validation(t *testing.T) {
	h := NewEarningHandler(nil, nil, nil, nil)

	// Missing earning_type
	req := httptest.NewRequest(http.MethodPost, "/api/v1/earnings", bytes.NewReader([]byte(`{"ticker": "PETR4", "gross_amount": "100"}`)))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
