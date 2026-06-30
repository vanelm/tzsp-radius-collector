package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vanelm/tzsp-radius-collector/internal/forwarder"
	"github.com/vanelm/tzsp-radius-collector/internal/recorder"
	"github.com/vanelm/tzsp-radius-collector/internal/replay"
	"github.com/vanelm/tzsp-radius-collector/internal/store"
	"github.com/vanelm/tzsp-radius-collector/internal/synth"
)

type Server struct {
	logger    *slog.Logger
	store     *store.Store
	forwarder *forwarder.Forwarder
	recorder  *recorder.Recorder
	replay    *replay.Engine
	synth     *synth.Engine
}

func New(
	logger *slog.Logger,
	st *store.Store,
	fwd *forwarder.Forwarder,
	rec *recorder.Recorder,
	rep *replay.Engine,
	syn *synth.Engine,
) *Server {
	return &Server{
		logger:    logger,
		store:     st,
		forwarder: fwd,
		recorder:  rec,
		replay:    rep,
		synth:     syn,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/forwarder", s.handleForwarder)
	mux.HandleFunc("/api/v1/catalog/export", s.handleCatalogExport)
	mux.HandleFunc("/api/v1/catalog/import", s.handleCatalogImport)
	mux.HandleFunc("/api/v1/nas", s.handleNASCollection)
	mux.HandleFunc("/api/v1/nas/", s.handleNASItem)
	mux.HandleFunc("/api/v1/clients", s.handleClientCollection)
	mux.HandleFunc("/api/v1/clients/", s.handleClientItem)
	mux.HandleFunc("/api/v1/recordings", s.handleRecordingCollection)
	mux.HandleFunc("/api/v1/recordings/start", s.handleRecordingStart)
	mux.HandleFunc("/api/v1/recordings/stop", s.handleRecordingStop)
	mux.HandleFunc("/api/v1/recordings/", s.handleRecordingItem)
	mux.HandleFunc("/api/v1/replay/stop", s.handleReplayStop)
	mux.HandleFunc("/api/v1/scenarios", s.handleScenarioCollection)
	mux.HandleFunc("/api/v1/scenarios/", s.handleScenarioItem)
	mux.HandleFunc("/api/v1/scenarios/run", s.handleScenarioRun)
	mux.HandleFunc("/api/v1/synth/stop", s.handleSynthStop)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	recActive, recID := s.recorder.Active()
	fwdTotal, fwdErrs := s.forwarder.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"forwarder": map[string]any{
			"config":          s.forwarder.GetConfig(),
			"forwarded_total": fwdTotal,
			"forward_errors":  fwdErrs,
		},
		"recorder": map[string]any{
			"active":       recActive,
			"recording_id": recID,
		},
		"replay": s.replay.Status(),
		"synth":  s.synth.Status(),
	})
}

func (s *Server) handleForwarder(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.forwarder.GetConfig())
	case http.MethodPut:
		var cfg forwarder.Config
		if err := decodeJSON(r, &cfg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.forwarder.SetConfig(cfg)
		writeJSON(w, http.StatusOK, cfg)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleNASCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListNAS()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item store.NAS
		if err := decodeJSON(r, &item); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		created, err := s.store.CreateNAS(item)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleNASItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/nas/")
	if id == "" || strings.Contains(id, "/") {
		notFound(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.store.GetNAS(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var item store.NAS
		if err := decodeJSON(r, &item); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item.ID = id
		if err := s.store.UpdateNAS(item); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.store.DeleteNAS(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleClientCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListClients()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var item store.Client
		if err := decodeJSON(r, &item); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		created, err := s.store.CreateClient(item)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleClientItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/clients/")
	if id == "" || strings.Contains(id, "/") {
		notFound(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.store.GetClient(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var item store.Client
		if err := decodeJSON(r, &item); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		item.ID = id
		if err := s.store.UpdateClient(item); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.store.DeleteClient(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleRecordingCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := s.store.ListRecordings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleRecordingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &body)
	}
	if body.Name == "" {
		body.Name = "recording"
	}
	rec, err := s.recorder.Start(body.Name)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleRecordingStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := s.recorder.Stop(); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRecordingItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/recordings/")
	if rest == "" {
		notFound(w)
		return
	}
	if strings.HasSuffix(rest, "/replay") {
		id := strings.TrimSuffix(rest, "/replay")
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		var opts replay.Options
		if r.ContentLength > 0 {
			if err := decodeJSON(r, &opts); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		} else {
			opts.RateRPS = 10
			opts.MirrorToStream = true
		}
		if opts.RateRPS <= 0 {
			opts.RateRPS = 10
		}
		if err := s.replay.Start(id, opts); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"recording_id": id})
		return
	}

	id := rest
	switch r.Method {
	case http.MethodGet:
		item, err := s.store.GetRecording(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.store.DeleteRecording(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleReplayStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	s.replay.Stop()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleScenarioCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.store.ListScenarios()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var body struct {
			Name   string `json:"name"`
			Config string `json:"config_json"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		sc, err := s.store.CreateScenario(store.Scenario{Name: body.Name, ConfigJSON: body.Config})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, sc)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleScenarioRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var cfg synth.RunConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.synth.Run(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleScenarioItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/scenarios/")
	if id == "" || strings.Contains(id, "/") {
		notFound(w)
		return
	}
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	if err := s.store.DeleteScenario(id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSynthStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	s.synth.Stop()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(body, dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, errors.New("not found"))
}
