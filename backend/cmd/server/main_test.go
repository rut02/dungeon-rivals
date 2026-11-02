package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"dungeon-rivals/backend/internal/config"
	"dungeon-rivals/backend/internal/dungeon"
	"dungeon-rivals/backend/internal/items"
	"dungeon-rivals/backend/internal/rooms"
)

func TestRoomHTTPFlow(t *testing.T) {
	var cfg config.Config
	cfg.Room.MinPlayers, cfg.Room.MaxPlayers = 2, 4
	cfg.Server.CORSAllowedOrigins = "http://localhost:5173"
	catalog, err := items.Load("../../data/items.json")
	if err != nil {
		t.Fatal(err)
	}
	dungeonConfig, err := dungeon.Load("../../data/dungeon.json")
	if err != nil {
		t.Fatal(err)
	}
	app := newApp(rooms.NewManagerWithGameData(2, 4, catalog, dungeonConfig), cfg, catalog)

	response, err := app.Test(httptest.NewRequest("GET", "/api/v1/health", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("health status = %d, want 200", response.StatusCode)
	}
	response, err = app.Test(httptest.NewRequest("GET", "/api/v1/items", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("items status = %d, want 200", response.StatusCode)
	}
	var definitions []items.Definition
	if err := json.NewDecoder(response.Body).Decode(&definitions); err != nil {
		t.Fatal(err)
	}
	if len(definitions) < 3 {
		t.Fatalf("catalog returned %d items, want at least 3", len(definitions))
	}

	create := httptest.NewRequest("POST", "/api/v1/rooms", strings.NewReader(`{"player_name":"Host"}`))
	create.Header.Set("Content-Type", "application/json")
	response, err = app.Test(create)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 {
		t.Fatalf("create status = %d, want 201", response.StatusCode)
	}
	var host rooms.SessionResponse
	if err := json.NewDecoder(response.Body).Decode(&host); err != nil {
		t.Fatal(err)
	}
	if !host.IsHost || host.RoomCode == "" || host.ReconnectToken == "" {
		t.Fatalf("invalid create response: %+v", host)
	}

	join := httptest.NewRequest("POST", "/api/v1/rooms/"+strings.ToLower(host.RoomCode)+"/join", strings.NewReader(`{"player_name":"Guest"}`))
	join.Header.Set("Content-Type", "application/json")
	response, err = app.Test(join)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 {
		t.Fatalf("join status = %d, want 201", response.StatusCode)
	}
	var guest rooms.SessionResponse
	if err := json.NewDecoder(response.Body).Decode(&guest); err != nil {
		t.Fatal(err)
	}
	if guest.IsHost || guest.RoomCode != host.RoomCode {
		t.Fatalf("invalid join response: %+v", guest)
	}
}
