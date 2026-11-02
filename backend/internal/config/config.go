package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Room struct {
		MinPlayers int `json:"min_players"`
		MaxPlayers int `json:"max_players"`
	} `json:"room"`
	Server struct {
		Port               string `json:"port"`
		CORSAllowedOrigins string `json:"cors_allowed_origins"`
	} `json:"server"`
}

// Load reads and validates the JSON configuration used by the local server.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Room.MinPlayers < 2 || cfg.Room.MaxPlayers > 4 || cfg.Room.MinPlayers > cfg.Room.MaxPlayers {
		return cfg, fmt.Errorf("room player limits must satisfy 2 <= min <= max <= 4")
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.CORSAllowedOrigins == "" {
		cfg.Server.CORSAllowedOrigins = "http://localhost:5173"
	}
	return cfg, nil
}
