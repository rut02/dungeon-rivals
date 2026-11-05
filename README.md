# Dungeon Rivals

Multiplayer dungeon roguelite for 2–4 players. The React + TypeScript + Vite frontend talks to a Go + Fiber server over HTTP and WebSocket. The server owns gameplay state and keeps active matches in memory; this MVP has no database.

## Requirements

- Node.js 20+
- Go 1.22+

## Run locally

In one PowerShell terminal:

```powershell
cd backend
go run ./cmd/server
```

In another:

```powershell
cd frontend
npm install
npm run dev
```

Open `http://localhost:5173`. The frontend proxies API and WebSocket traffic to `http://localhost:8080`. Health check: `http://localhost:8080/api/v1/health`.

## Game flow

1. Create or join a room, ready up, and let the host start the match.
2. Each player explores a private, branching three-floor dungeon. Combat, treasure, shops, events, rest rooms, respawn, and checkpoints resolve independently per player.
3. After clearing the final boss, review equipment and ready for the Arena. The server restores HP and starts PvP when every remaining player is ready.
4. Players fight each scheduled opponent in a round-robin Arena. Turns alternate, each bout ends when a fighter reaches 0 HP, and final ranking is calculated after every bout is complete. Ranking uses PvE score, PvP score, Gold conversion, bonuses, and penalties. Results can be downloaded as CSV.

The final art pass adds original 64-bit Dark Fantasy Pixel Artwork for characters, equipment, monsters, and covers in `frontend/public/art/pixel/` and `frontend/public/art/`. See [docs/HOWTO_PROCEDURAL_PIXEL_ART.md](docs/HOWTO_PROCEDURAL_PIXEL_ART.md) for the procedural sprite generation guide and tools.

## Static data and configuration

When started from `backend`, the server loads:

- `config/game.json`
- `data/items.json`
- `data/dungeon.json`
- `data/monsters.json`

Override paths and network settings with `CONFIG_PATH`, `ITEMS_PATH`, `DUNGEON_PATH`, `MONSTERS_PATH`, `PORT`, and `CORS_ALLOWED_ORIGINS`. Root `.env.example` lists local defaults; environment files are not loaded automatically.

## Validation

```powershell
cd backend
go test ./...
go test -race ./...
cd ../frontend
npm run build
```

## Runtime limits

Rooms, results, idempotency records, and score events live in server memory. Restarting the backend clears them. The optional CSV endpoint is available after a match at `/api/v1/rooms/{ROOM_CODE}/results.csv`.
