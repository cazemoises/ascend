package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "returns 200 with ok status",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			Health(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.wantBody {
				t.Errorf("got body %q, want %q", got, tt.wantBody)
			}
		})
	}
}
