package rooms

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

const maxMessageSize = 16 * 1024

type safeWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *safeWriter) WriteJSON(value any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return w.conn.WriteJSON(value)
}

type message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type authPayload struct {
	RoomCode       string `json:"room_code"`
	PlayerID       string `json:"player_id"`
	ReconnectToken string `json:"reconnect_token"`
}

func envelope(kind, requestID string, payload any) map[string]any {
	return map[string]any{"type": kind, "request_id": requestID, "payload": payload}
}

// ServeWebSocket authenticates one player session before accepting lobby commands.
func ServeWebSocket(manager *Manager, conn *websocket.Conn) {
	defer conn.Close()
	writer := &safeWriter{conn: conn}
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var first message
	if err := conn.ReadJSON(&first); err != nil || first.Type != "session.authenticate" {
		_ = writer.WriteJSON(envelope("command.error", first.RequestID, map[string]string{"code": "AUTHENTICATION_REQUIRED", "message": "Authenticate before sending commands"}))
		return
	}
	if first.RequestID == "" || len(first.RequestID) > 128 {
		writeError(writer, first.RequestID, ErrInvalidRequestID.Error())
		return
	}
	var auth authPayload
	if err := json.Unmarshal(first.Payload, &auth); err != nil {
		writeError(writer, first.RequestID, "INVALID_REQUEST")
		return
	}
	r, p, err := manager.authenticate(strings.ToUpper(auth.RoomCode), auth.PlayerID, auth.ReconnectToken, writer)
	if err != nil {
		writeError(writer, first.RequestID, err.Error())
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	if err := writer.WriteJSON(envelope("session.authenticated", first.RequestID, map[string]any{"player_id": p.ID, "room_code": r.Code})); err != nil {
		manager.disconnect(r, p.ID, writer, false)
		return
	}
	manager.notifyConnected(r, p.ID)
	for {
		var incoming message
		if err := conn.ReadJSON(&incoming); err != nil {
			break
		}
		if err := manager.ClaimRequestID(r, p.ID, incoming.RequestID); err != nil {
			writeError(writer, incoming.RequestID, err.Error())
			continue
		}
		var payload map[string]any
		if len(incoming.Payload) > 0 {
			if err := json.Unmarshal(incoming.Payload, &payload); err != nil {
				writeError(writer, incoming.RequestID, "INVALID_REQUEST")
				continue
			}
		}
		if payload == nil {
			payload = map[string]any{}
		}
		if incoming.Type == "session.authenticate" {
			writeError(writer, incoming.RequestID, "ALREADY_AUTHENTICATED")
			continue
		}
		err := manager.ServeCommand(r, p.ID, writer, incoming.Type, payload)
		if err != nil {
			writeError(writer, incoming.RequestID, err.Error())
			continue
		}
		if incoming.Type == "room.leave" {
			return
		}
		_ = writer.WriteJSON(envelope("command.accepted", incoming.RequestID, map[string]string{"message": "Action accepted"}))
	}
	manager.disconnect(r, p.ID, writer, false)
}

func (m *Manager) sendSnapshot(r *room, playerID string, conn interface{ WriteJSON(any) error }) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rooms[r.Code] == r {
		_ = conn.WriteJSON(envelope("room.snapshot", "", m.snapshotLocked(r, playerID)))
	}
}

func writeError(conn interface{ WriteJSON(any) error }, requestID, code string) {
	message := code
	if code == "INVALID_REQUEST" {
		message = "Request payload is invalid"
	}
	_ = conn.WriteJSON(envelope("command.error", requestID, map[string]string{"code": code, "message": message}))
}
