package stream

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/vanelm/tzsp-radius-collector/internal/pipeline"
)

type Filter struct {
	Codes       []string `json:"codes,omitempty"`
	UserNames   []string `json:"user_names,omitempty"`
	NASIPs      []string `json:"nas_ips,omitempty"`
	Vendors     []string `json:"vendors,omitempty"`
	MACs        []string `json:"macs,omitempty"`
	StatusTypes []string `json:"status_types,omitempty"`
	Sources     []string `json:"sources,omitempty"`
}

type Client struct {
	id     string
	send   chan []byte
	filter Filter
}

type Hub struct {
	logger    *slog.Logger
	queueSize int

	mu      sync.RWMutex
	clients map[string]*Client
}

func NewHub(logger *slog.Logger, queueSize int) *Hub {
	return &Hub{
		logger:    logger,
		queueSize: queueSize,
		clients:   map[string]*Client{},
	}
}

func (h *Hub) Run(ctx context.Context) {
	<-ctx.Done()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, client := range h.clients {
		close(client.send)
	}
	h.clients = map[string]*Client{}
}

func (h *Hub) Add(id string) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	client := &Client{id: id, send: make(chan []byte, h.queueSize)}
	h.clients[id] = client
	h.logger.Debug("client connected", "client_id", id, "total", len(h.clients))
	return client
}

func (h *Hub) Remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	client := h.clients[id]
	if client == nil {
		return
	}
	delete(h.clients, id)
	close(client.send)
	h.logger.Debug("client disconnected", "client_id", id, "total", len(h.clients))
}

func (h *Hub) UpdateFilter(id string, filter Filter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client := h.clients[id]; client != nil {
		client.filter = filter
		h.logger.Debug("filter updated", "client_id", id, "filter", filter)
	}
}

func (h *Hub) Publish(msg pipeline.StreamMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) == 0 {
		return
	}
	matched := false
	for _, client := range h.clients {
		if matchesFilter(msg, client.filter) {
			matched = true
			break
		}
	}
	if !matched {
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for _, client := range h.clients {
		if !matchesFilter(msg, client.filter) {
			continue
		}
		select {
		case client.send <- payload:
		default:
			h.logger.Warn("disconnecting slow client", "client_id", client.id)
			go h.Remove(client.id)
		}
	}
}

func matchesFilter(msg pipeline.StreamMessage, filter Filter) bool {
	if len(filter.Codes) > 0 && !containsFold(filter.Codes, msg.Radius.CodeName) {
		return false
	}
	accounting := msg.Accounting
	if len(filter.UserNames) > 0 && !containsFold(filter.UserNames, mapString(accounting, "user_name")) {
		return false
	}
	if len(filter.NASIPs) > 0 && !containsFold(filter.NASIPs, mapString(accounting, "nas")) {
		return false
	}
	if len(filter.StatusTypes) > 0 && !containsFold(filter.StatusTypes, mapString(accounting, "status_type")) {
		return false
	}
	if len(filter.MACs) > 0 && !containsFold(filter.MACs, mapString(accounting, "mac")) {
		return false
	}
	if len(filter.Vendors) > 0 && !containsFold(filter.Vendors, mapString(accounting, "nas_vendor")) {
		return false
	}
	if len(filter.Sources) > 0 && !containsFold(filter.Sources, msg.Source) {
		return false
	}
	return true
}
