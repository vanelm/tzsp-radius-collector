package stream

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vanelm/tzsp-radius-collector/internal/config"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type subscribeMessage struct {
	Action string `json:"action"`
	Filter Filter `json:"filter"`
}

func RegisterRoutes(mux *http.ServeMux, hub *Hub, cfg config.Config) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc(cfg.WSPath, func(w http.ResponseWriter, r *http.Request) {
		handleWS(hub, w, r)
	})
}

func handleWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	id := strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "")
	client := hub.Add(id)
	defer hub.Remove(id)

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(2*time.Second))
			}
		}
	}()

	go func() {
		for {
			payload, ok := <-client.send
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}()

	for {
		_, data, readErr := conn.ReadMessage()
		if readErr != nil {
			return
		}
		var incoming subscribeMessage
		if err := json.Unmarshal(data, &incoming); err != nil {
			continue
		}
		if strings.EqualFold(incoming.Action, "subscribe") {
			hub.UpdateFilter(id, incoming.Filter)
		}
	}
}
