package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vanelm/tzsp-radius-collector/internal/store"
)

func (s *Server) handleCatalogExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	data, err := s.store.ExportCatalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	filename := fmt.Sprintf("radius-catalog-%s.json", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func (s *Server) handleCatalogImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("empty body"))
		return
	}

	var req store.CatalogImportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Mode == "" {
		req.Mode = strings.TrimSpace(r.URL.Query().Get("mode"))
	}
	if len(req.NAS) == 0 && len(req.Clients) == 0 {
		var export store.CatalogExport
		if err := json.Unmarshal(body, &export); err == nil {
			req.NAS = export.NAS
			req.Clients = export.Clients
			if req.Mode == "" {
				req.Mode = "merge"
			}
		}
	}

	result, err := s.store.ImportCatalog(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
