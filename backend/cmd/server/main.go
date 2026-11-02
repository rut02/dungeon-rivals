package main

import (
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"

	"dungeon-rivals/backend/internal/combat"
	"dungeon-rivals/backend/internal/config"
	"dungeon-rivals/backend/internal/dungeon"
	"dungeon-rivals/backend/internal/items"
	"dungeon-rivals/backend/internal/rooms"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/game.json"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	itemsPath := os.Getenv("ITEMS_PATH")
	if itemsPath == "" {
		itemsPath = "data/items.json"
	}
	catalog, err := items.Load(itemsPath)
	if err != nil {
		log.Fatalf("load items: %v", err)
	}
	dungeonPath := os.Getenv("DUNGEON_PATH")
	if dungeonPath == "" {
		dungeonPath = "data/dungeon.json"
	}
	dungeonConfig, err := dungeon.Load(dungeonPath)
	if err != nil {
		log.Fatalf("load dungeon: %v", err)
	}
	monstersPath := os.Getenv("MONSTERS_PATH")
	if monstersPath == "" {
		monstersPath = "data/monsters.json"
	}
	monsters, err := combat.Load(monstersPath)
	if err != nil {
		log.Fatalf("load monsters: %v", err)
	}
	manager := rooms.NewManagerWithAllGameData(cfg.Room.MinPlayers, cfg.Room.MaxPlayers, catalog, dungeonConfig, monsters)
	app := newApp(manager, cfg, catalog)
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Server.Port
	}
	log.Fatal(app.Listen("127.0.0.1:" + port))
}

func newApp(manager *rooms.Manager, cfg config.Config, catalog *items.Catalog) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Dungeon Rivals API"})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{AllowOrigins: allowedOrigins(cfg.Server.CORSAllowedOrigins), AllowHeaders: "Origin, Content-Type, Accept", AllowMethods: "GET,POST,OPTIONS"}))

	app.Get("/api/v1/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/api/v1/items", func(c *fiber.Ctx) error {
		return c.JSON(catalog.List())
	})
	app.Post("/api/v1/rooms", func(c *fiber.Ctx) error {
		var request rooms.PlayerNameRequest
		if err := c.BodyParser(&request); err != nil {
			return apiError(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
		}
		result, err := manager.Create(request.PlayerName)
		if err != nil {
			return roomError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	})
	app.Post("/api/v1/rooms/:room_code/join", func(c *fiber.Ctx) error {
		var request rooms.PlayerNameRequest
		if err := c.BodyParser(&request); err != nil {
			return apiError(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Request body must be valid JSON")
		}
		result, err := manager.Join(strings.ToUpper(c.Params("room_code")), request.PlayerName)
		if err != nil {
			return roomError(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	})
	app.Get("/api/v1/rooms/:room_code", func(c *fiber.Ctx) error {
		preview, err := manager.Preview(strings.ToUpper(c.Params("room_code")))
		if err != nil {
			return roomError(c, err)
		}
		return c.JSON(preview)
	})
	app.Get("/api/v1/rooms/:room_code/results.csv", func(c *fiber.Ctx) error {
		body, err := manager.ResultsCSV(strings.ToUpper(c.Params("room_code")))
		if err != nil {
			return roomError(c, err)
		}
		c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
		c.Set(fiber.HeaderContentDisposition, "attachment; filename=\"dungeon-rivals-results.csv\"")
		return c.Send(body)
	})

	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		rooms.ServeWebSocket(manager, conn)
	}))

	return app
}

func allowedOrigins(configured string) string {
	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if origins == "" {
		return configured
	}
	return origins
}

func roomError(c *fiber.Ctx, err error) error {
	status := fiber.StatusBadRequest
	code := err.Error()
	switch code {
	case rooms.ErrRoomNotFound.Error():
		status = fiber.StatusNotFound
	case rooms.ErrRoomFull.Error(), rooms.ErrRoomStarted.Error():
		status = fiber.StatusConflict
	}
	return apiError(c, status, code, code)
}

func apiError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"code": code, "message": message})
}
