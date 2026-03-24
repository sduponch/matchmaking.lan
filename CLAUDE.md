# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CS2 LAN matchmaking platform. Three components:

- `frontend/` — Vue 3 dashboard (hud-vue v6.0.0, Bootstrap 5, Vite)
- `backend/` — Go REST API (Gin), Steam OpenID auth, JWT, Faceit API
- `backend/bot/` — Node.js subprocess, Steam bot for CS2 rank fetching via Game Coordinator

## Commands

### Frontend (`frontend/`)
```bash
npm install
npm run dev          # Vite dev server
npm run build        # Type check + production build
npm run preview      # Preview on port 5050
npm run lint
npm run typecheck
```

### Backend (`backend/`)
```bash
go build ./...
go run ./cmd/server/main.go   # Requires .env (copy from .env.example)
```

### Bot (`backend/bot/`)
```bash
npm install
# Started automatically by the Go backend as a subprocess
```

## Environment

`backend/.env` (see `.env.example`):
- `PORT`, `FRONTEND_URL`, `BACKEND_URL`
- `STEAM_API_KEY`, `ADMIN_STEAM_IDS`
- `JWT_SECRET`, `JWT_EXPIRY`
- `FACEIT_API_KEY`
- `BOT_PORT`, `BOT_USERNAME`, `BOT_PASSWORD`, `BOT_SHARED_SECRET`

## Architecture

### Auth flow
Steam OpenID URL built client-side in `PageLogin.vue` → popup window → Go callback `/auth/steam` validates OpenID + issues JWT → `postMessage` to parent → Pinia `auth` store persists token in localStorage.

JWT claims: `steamid`, `username`, `avatar`, `role` (`admin` ou `player`).

Admins définis via `ADMIN_STEAM_IDS` dans `.env`.

### API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/steam` | — | Steam OpenID callback |
| GET | `/auth/me` | ✓ | Infos JWT du joueur connecté |
| GET | `/profile/:steamid` | ✓ | Profil Steam + stats cachées (cs2_status / faceit_status) |
| GET | `/profile/:steamid/cs2` | ✓ | Stats CS2 (polling, pending si fetch en cours) |
| GET | `/profile/:steamid/faceit` | ✓ | Stats Faceit (polling, pending si fetch en cours) |
| GET | `/servers` | ✓ | Liste serveurs LAN (découverte A2S + gérés) |
| POST | `/servers` | admin | Ajouter serveur géré (teste RCON, configure logaddress_add_http) |
| DELETE | `/servers/:addr` | admin | Retirer serveur géré |
| POST | `/servers/:addr/map` | admin | Changer la map (RCON changelevel) |
| GET | `/servers/:addr/match` | ✓ | État du match en cours |
| GET | `/servers/:addr/logs` | ✓ | SSE stream des événements de log |
| POST | `/internal/log` | — | Réception logs HTTP CS2 (logaddress_add_http) |

### Stats cache (`backend/internal/player/`)

Les stats CS2 et Faceit sont fetchées en background et mises en cache séparément.

**cs2_status** : `retrieving` → `ready` / `pending_invite` / `unavailable`
**faceit_status** : `retrieving` → `ready` / `not_found` / `unavailable`

TTL : `ready` = 5 min, `pending_invite` = 2 min, `unavailable` = 30 s, `not_found` = 10 min.

### Serveurs LAN (`backend/internal/server/`)

- **Découverte A2S** : broadcast UDP `255.255.255.255:27015`, réponse `go-a2s`, déduit joueurs humains = `Players - Bots`
- **Store** : `servers.json` — `map[addr]rconPassword`, persisté sur disque
- **Ajout** : teste RCON avant d'enregistrer, envoie `logaddress_add_http` pour activer les logs HTTP
- **RCON** : `gorcon/rcon`, utilisé pour `changelevel <map>`

### Bot CS2 (`backend/bot/index.js`)

Subprocess Node.js lancé par `backend/internal/bot/manager.go` (redémarre automatiquement).

- Se connecte à Steam + CS2 Game Coordinator
- Expose HTTP sur `127.0.0.1:BOT_PORT` : `GET /rank/:steamid`, `GET /info`
- Si GC timeout → envoie une demande d'ami (`client.addFriend`) + retourne `pending_invite`
- Accepte automatiquement les demandes d'amis entrantes

Le bot doit avoir CS2 Premium (compte payant) pour pouvoir envoyer des demandes d'amis.

### Gamelog (`backend/internal/gamelog/`)

Réception des logs CS2 via HTTP POST (`logaddress_add_http`).

**Format CS2 :** `MM/DD/YYYY - HH:MM:SS.mmm - <message>`

**Pattern registry DSL (`patterns.json`) :**
- Tokens `{name:type}` compilés en regex nommés
- Types : `player` (nom+uid+steamid+team), `player_nt` (sans team), `quoted`, `int`, `word`, `pos`
- Événements dot-notation : `cs2.kill`, `cs2.kill.headshot`, `cs2.round.start`, `cs2.bomb.plant`, etc.

**JSON blocks :** CS2 envoie des blocs `JSON_BEGIN{...}}JSON_END` contenant les stats de fin de round (`round_stats`) avec les stats CSV par joueur.

**SteamID :** les logs CS2 utilisent le format Steam3 `[U:1:160633]`. Converti en Steam64 via `steam3ToSteam64` : `steam64 = 76561197960265728 + accountid`.

**SSE broker :** `gamelog.Broker` diffuse les événements par serveur. `gamelog.OnEvent` hook pour brancher la machine d'état.

### Machine d'état (`backend/internal/match/`)

Par serveur, suit : `phase`, `map`, `round`, `score_ct`, `score_t`, `players`.

**Phases :** `idle` → `warmup` → `live` → `game_over`

**`PlayerStat` :** `name`, `steamid` (Steam64), `team`, `kills`, `deaths`, `assists`, `dmg`, `hsp`, `adr`, `money`, `mvp`

Stats enrichies à chaque `cs2.round.stats` (JSON block) — `findByAccountID` convertit l'accountid 32-bit en Steam64 pour la lookup directe.

### Frontend (`frontend/src/`)

**Stores Pinia :**
- `auth.ts` — token JWT, user (steamid, username, avatar, role), `init()` / `setToken()` / `logout()`
- `app-option.ts` — état UI layout (sidebar, header, thème…)

**Vues :**
- `PageLogin.vue` — bouton login Steam (popup OpenID)
- `PageAuthDone.vue` — récepteur postMessage après auth
- `Profile.vue` — profil joueur, poll `/cs2` et `/faceit` séparément tant que `retrieving`
- `Home.vue` — liste serveurs LAN, état match en temps réel (score, tableau des joueurs avec avatar/K/D/A/DMG/ADR/$/MVP), contrôles admin (RCON, changement de map)

**Config :** `src/config/matchmaking.ts` — `api.baseUrl`, `admins.steamIds`

**Ne jamais modifier** `src/env.d.ts`.

### Layout system
`App.vue` conditionne Header + Sidebar + `<RouterView>` + Footer via `appOptionStore`. `@` alias → `src/`.

### Styling
Bootstrap 5, SCSS dans `src/scss/`. Dark mode par défaut (`data-bs-theme="dark"`).
