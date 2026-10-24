package handler

import (
	_ "embed"
	"net/http"
)

// Only served behind teacher authentication; hidden tests must never enter
// the public frontend bundle or the anonymous challenge/list payloads.
//
//go:embed activity_templates/fastapi-v1.json
var fastAPISequence []byte

func (h *ListsHandler) HTTPSequence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Write(fastAPISequence)
}
