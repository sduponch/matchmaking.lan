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

`FRONTEND_URL` and `BACKEND_URL` must include scheme (`https://` or `http://`). Missing scheme causes broken redirect URLs in the Steam auth callback.

`BACKEND_URL` is used to build the `logaddress_add_http` URL sent to CS2 servers via RCON. CS2 supports both `http://` and `https://`.

## Architecture

### Auth flow
Steam OpenID URL built client-side in `PageLogin.vue` → popup window → Go callback `/auth/steam` validates OpenID + issues JWT → `postMessage` to parent → Pinia `auth` store persists token in localStorage.

JWT claims: `steamid`, `username`, `avatar`, `role` (`admin` ou `player`).

Admins définis via `ADMIN_STEAM_IDS` dans `.env`.

On successful login, `registry.Upsert()` is called to register/update the player in `players.json`.

### API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/steam` | — | Steam OpenID callback |
| GET | `/auth/me` | ✓ | Infos JWT du joueur connecté |
| GET | `/profile/:steamid` | ✓ | Profil Steam + stats cachées (cs2_status / faceit_status) |
| GET | `/profile/:steamid/cs2` | ✓ | Stats CS2 (polling, pending si fetch en cours) |
| GET | `/profile/:steamid/faceit` | ✓ | Stats Faceit (polling, pending si fetch en cours) |
| GET | `/players` | admin | Liste joueurs enregistrés (registry) |
| GET | `/teams` | ✓ | Liste équipes |
| POST | `/teams` | admin | Créer une équipe |
| DELETE | `/teams/:id` | admin | Supprimer une équipe |
| POST | `/teams/:id/players` | admin | Ajouter joueur à l'équipe |
| DELETE | `/teams/:id/players/:steamid` | admin | Retirer joueur de l'équipe |
| GET | `/servers` | ✓ | Liste serveurs LAN (découverte A2S + gérés) |
| POST | `/servers` | admin | Ajouter serveur géré (teste RCON, configure logaddress_add_http) |
| DELETE | `/servers/:addr` | admin | Retirer serveur géré |
| POST | `/servers/:addr/map` | admin | Changer la map (RCON changelevel) |
| GET | `/servers/:addr/match` | ✓ | État du match en cours + last_log_at |
| GET | `/servers/:addr/logs` | ✓ | SSE stream des événements de log |
| POST | `/internal/log/:token` | — | Réception logs CS2 (logaddress_add_http, token-based) |
| POST | `/internal/log` | — | Réception logs CS2 (fallback sans token) |

### Persistance

| Fichier | Contenu |
|---------|---------|
| `servers.json` | `map[addr]{rcon, token}` — token 16 octets hex aléatoire par serveur |
| `players.json` | `map[steamid]{username, avatar, role, team, last_seen}` |
| `teams.json` | `map[id]{name, players[]}` |

`last_log_at` est **en mémoire uniquement** — pas de persistance disque pour éviter les écritures à chaque log.

**Migration `servers.json`** : si l'ancien format `map[string]string` est détecté, il est migré automatiquement vers le nouveau format avec génération de tokens.

### Registre joueurs (`backend/internal/registry/`)

- `store.go` : `Player{SteamID, Username, Avatar, Role, Team, LastSeen}`, persisté dans `players.json`
- `Upsert(steamid, username, avatar, role)` — appelé à chaque connexion
- `SetTeam(steamid, teamName)` — appelé par `teams` pour synchroniser le champ équipe
- `List()` — trié par `last_seen` décroissant
- `handler.go` : `GET /players` (admin uniquement)

### Gestion équipes (`backend/internal/teams/`)

- `store.go` : `Team{ID, Name, Players[], CreatedAt}`, persisté dans `teams.json`
- `newID()` — génère un ID hex aléatoire
- `AddPlayer()` / `RemovePlayer()` — appellent `registry.SetTeam()` pour synchronisation bidirectionnelle
- `HandleDelete` — efface le champ équipe de tous les membres avant suppression
- `handler.go` : 5 handlers (List, Create, Delete, AddPlayer, RemovePlayer)

### Stats cache (`backend/internal/player/`)

Les stats CS2 et Faceit sont fetchées en background et mises en cache séparément.

**cs2_status** : `retrieving` → `ready` / `pending_invite` / `unavailable`
**faceit_status** : `retrieving` → `ready` / `not_found` / `unavailable`

TTL : `ready` = 5 min, `pending_invite` = 2 min, `unavailable` = 30 s, `not_found` = 10 min.

### Serveurs LAN (`backend/internal/server/`)

- **Découverte A2S** : broadcast UDP `255.255.255.255:27015`, réponse `go-a2s`, déduit joueurs humains = `Players - Bots`
- **Store** : `servers.json` — `map[addr]{rcon, token}`, token 16 octets hex aléatoire par serveur
- **Ajout** : teste RCON avant d'enregistrer → enregistre `ExpectLog` goroutine → envoie `log on` → envoie `logaddress_add_http "BACKEND_URL/internal/log/TOKEN"` → attend confirmation réception (timeout 5s)
- **`last_log_at`** : en mémoire (`map[string]time.Time`), mis à jour à chaque POST reçu, exposé via `GetLastLogAt(addr)`
- **`GetAddrByToken(token)`** : résout un token en adresse serveur — utilisé par `gamelog.ResolveToken`
- **RCON** : `gorcon/rcon`, utilisé pour `changelevel <map>`

### Identification serveur derrière NAT

CS2 envoie les logs HTTP depuis l'IP de la passerelle, pas l'IP individuelle des serveurs. Solution : chaque serveur a un token unique (16 octets hex) embarqué dans le path de l'URL `logaddress_add_http`. Le token est résolu en adresse serveur à chaque POST reçu.

```
logaddress_add_http "https://api.example.com/internal/log/<token>"
```

### Hook pattern (évite les imports circulaires)

Les packages `gamelog`, `server` et `match` ne peuvent pas s'importer mutuellement. Les dépendances sont câblées dans `main.go` via des variables de fonction :

```go
gamelog.OnEvent      = match.Apply           // dispatch événements → machine d'état
gamelog.ResolveToken = server.GetAddrByToken // résolution token → addr
gamelog.OnLog        = server.UpdateLastLog  // mise à jour last_log_at
match.GetLastLogAt   = server.GetLastLogAt   // inclus dans GET /servers/:addr/match
```

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

**Hooks exportés :**
- `OnEvent func(*Event)` — déclenché pour chaque événement parsé
- `OnLog func(addr string)` — déclenché à chaque POST reçu (avant parsing), pour `last_log_at`
- `ResolveToken func(token string) (addr string, ok bool)` — résout token → addr serveur

**`ExpectLog(addr, timeout)`** — attend un log entrant pour un serveur donné, utilisé pour vérifier la réception après `logaddress_add_http`.

### Machine d'état (`backend/internal/match/`)

Par serveur, suit : `phase`, `map`, `round`, `score_ct`, `score_t`, `players`.

**Phases :** `idle` → `warmup` → `live` → `game_over`

**`PlayerStat` :** `name`, `steamid` (Steam64), `team`, `kills`, `deaths`, `assists`, `dmg`, `hsp`, `adr`, `money`, `mvp`

Stats enrichies à chaque `cs2.round.stats` (JSON block) — `findByAccountID` convertit l'accountid 32-bit en Steam64 pour la lookup directe.

`GET /servers/:addr/match` retourne l'état + `last_log_at` (via hook `GetLastLogAt`). Le frontend poll cet endpoint toutes les 5s pour mettre à jour `last_log_at` sans attendre `fetchServers()`.

**Hook exporté :**
- `GetLastLogAt func(addr string) *time.Time` — câblé vers `server.GetLastLogAt`

### Frontend (`frontend/src/`)

**Stores Pinia :**
- `auth.ts` — token JWT, user (steamid, username, avatar, role), `init()` / `setToken()` / `logout()`
- `app-option.ts` — état UI layout (sidebar, header, thème…)

**Vues admin :**
- `AdminPlayers.vue` — tableau joueurs avec avatar/rôle/équipe/stats CS2+Faceit/dernière connexion, poll `retrieving`
- `AdminMatchmakingTeams.vue` — accordion : liste équipes avec moyennes CS2/Faceit, composition, ajout/retrait joueurs, création équipe (modal)
- `AdminServersManage.vue` — tableau serveurs avec colonne "Dernier log" (vert <2min, orange 2-10min, rouge >10min)
- `AdminServerSetup.vue` — ajout serveur avec vérification réception logs

**Vues joueur :**
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

## Notes techniques importantes

- **`go build ./...`** est la seule façon fiable de vérifier le code Go — gopls affiche de faux positifs car `go.work` a été supprimé (gin v1.12.0 requiert go 1.25.0, système a go 1.24.4).
- **`go.work` supprimé** — ne pas recréer, gopls ne fonctionne pas correctement dans ce contexte.
- **`logaddress_add_http`** — n'accepte qu'un seul paramètre URI. Le token doit être dans le path, pas en second argument. CS2 supporte `http://` et `https://`.
- **Imports circulaires** — `gamelog`, `server`, `match` ne peuvent pas s'importer entre eux. Utiliser le pattern hook via `main.go`.
