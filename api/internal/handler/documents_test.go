package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pircos/api/internal/service"
)

func TestDocumentHandler_Parse(t *testing.T) {
	parser := service.NewSinacorParser()
	h := NewDocumentHandler(parser, nil, nil, nil, nil, nil, nil)

	sampleNote := `NOTA DE CORRETAGEM
Nr. nota: 554433
Data pregão: 20/02/2024
Líquido para: 22/02/2024

1-BOVESPA C VISTA PETR4 PETROBRAS PN 100 30,00 3.000,00 D

Taxa de liquidação: 1,00
Emolumentos: 0,50
Total custos / despesas: 1,50
`

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "nota_teste.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write([]byte(sampleNote)); err != nil {
		t.Fatalf("failed to write part: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/parse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Parse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d. body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var note service.ParsedBrokerageNote
	if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if note.NoteNumber != "554433" {
		t.Errorf("NoteNumber = %s, want 554433", note.NoteNumber)
	}
	if len(note.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(note.Transactions))
	}
	if note.Transactions[0].Ticker != "PETR4" {
		t.Errorf("Ticker = %s, want PETR4", note.Transactions[0].Ticker)
	}
}

func TestDocumentHandler_Parse_MissingFile(t *testing.T) {
	parser := service.NewSinacorParser()
	h := NewDocumentHandler(parser, nil, nil, nil, nil, nil, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/parse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Parse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
