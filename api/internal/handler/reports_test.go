package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaxHandler_Drilldown_MissingParams(t *testing.T) {
	h := NewTaxHandler(nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tax/drilldown", nil)
	rec := httptest.NewRecorder()

	h.Drilldown(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestBenchmarkHandler_List(t *testing.T) {
	h := NewBenchmarkHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/benchmarks", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
