# matchmaking.lan

Plateforme de matchmaking CS2 pour LAN. Tableau de bord en temps réel avec suivi des matchs, gestion des serveurs et authentification Steam.

## Composants

| Composant | Stack | Rôle |
|-----------|-------|------|
| `frontend/` | Vue 3, Bootstrap 5, Vite | Dashboard joueurs & admin |
| `backend/` | Go, Gin | API REST, auth Steam, logique match |
| `backend/bot/` | Node.js | Fetch ranks CS2 via Game Coordinator |

## Démarrage rapide

### Prérequis

- Go 1.21+
- Node.js 18+
- Un serveur CS2 sur le réseau local
- Clé API Steam ([steamcommunity.com/dev/apikey](https://steamcommunity.com/dev/apikey))

### Backend

```bash
cd backend
cp .env.example .env
# Éditer .env (voir section Configuration)
go run ./cmd/server/main.go
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

## Configuration

`backend/.env` :

```env
PORT=8080
FRONTEND_URL=http://localhost:5173
BACKEND_URL=http://localhost:8080

# Steam
STEAM_API_KEY=votre_cle_api_steam
ADMIN_STEAM_IDS=76561197960426361,76561197960000000

# JWT
JWT_SECRET=votre_secret_jwt
JWT_EXPIRY=24h

# Faceit (optionnel)
FACEIT_API_KEY=votre_cle_faceit

# Bot Steam (optionnel, pour fetch ranks CS2)
BOT_PORT=3001
BOT_USERNAME=compte_steam_bot
BOT_PASSWORD=mot_de_passe_bot
BOT_SHARED_SECRET=shared_secret_2fa
```

## Fonctionnalités

### Authentification
- Login via Steam OpenID (popup)
- JWT avec rôle `admin` ou `player`
- Admins définis par Steam64 ID dans `.env`

### Profil joueur
- Informations Steam (avatar, niveau, statut)
- Rang CS2 Premier / Compétitif (via bot Game Coordinator)
- Stats Faceit (ELO, niveau, K/D, win rate…)
- Cache avec TTL adaptatif par statut

### Serveurs LAN
- Découverte automatique par broadcast UDP (protocole A2S)
- Ajout manuel avec mot de passe RCON
- Changement de map à distance
- Comptage joueurs humains (hors bots)

### Suivi de match en temps réel
- Réception des logs CS2 via `logaddress_add_http`
- Parser de logs DSL avec pattern registry (`patterns.json`)
- Machine d'état par serveur : phase, score CT/T, round, map
- Tableau des scores par équipe : K/D/A, DMG, ADR, argent, MVP
- Avatars Steam dans le tableau
- Mise à jour toutes les 5 secondes + SSE pour streaming temps réel

### Formats CS2 supportés
- Kills (normal, headshot, bombe, suicide)
- Rounds (start, end, stats JSON)
- Bombe (plant begin, planted, defused, exploded)
- Joueurs (connect, disconnect, switch équipe)
- Chat (global, équipe)
- Score, statut match, warmup, freeze period

## Architecture technique

### Gamelog pipeline

```
CS2 server
    │ logaddress_add_http POST
    ▼
/internal/log (HTTPHandler)
    │ parse timestamp + content
    │ JSON_BEGIN...JSON_END → parseJSONBlock()
    │ pattern registry (patterns.json DSL)
    ▼
gamelog.Event {Type, Server, At, Fields, Extra}
    │
    ├─→ gamelog.Broker.publish() → SSE /servers/:addr/logs
    └─→ gamelog.OnEvent() → match.Apply()
                                │
                                ▼
                        match.Machine (par serveur)
                        MatchState {phase, map, round, score, players}
```

### SteamID

Les logs CS2 utilisent le format Steam3 `[U:1:160633]` et les JSON blocks utilisent l'`accountid` 32-bit (`160633`). Tous deux sont convertis en Steam64 via :

```
steam64 = 76561197960265728 + accountid
```

Tous les `PlayerStat.SteamID` sont stockés en Steam64, ce qui aligne avec les profils Steam et les JWT.

### Pattern DSL

```json
{ "type": "cs2.kill.headshot", "pattern": "{killer:player} {kpos:pos} killed {victim:player} {vpos:pos} with {weapon:quoted} (headshot)" }
```

Tokens disponibles : `player`, `player_nt`, `quoted`, `int`, `word`, `pos`

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/steam` | — | Callback OpenID Steam |
| GET | `/auth/me` | ✓ | Infos joueur connecté |
| GET | `/profile/:steamid` | ✓ | Profil Steam complet |
| GET | `/profile/:steamid/cs2` | ✓ | Stats CS2 |
| GET | `/profile/:steamid/faceit` | ✓ | Stats Faceit |
| GET | `/servers` | ✓ | Liste serveurs LAN |
| POST | `/servers` | admin | Ajouter serveur (RCON requis) |
| DELETE | `/servers/:addr` | admin | Retirer serveur |
| POST | `/servers/:addr/map` | admin | Changer la map |
| GET | `/servers/:addr/match` | ✓ | État du match |
| GET | `/servers/:addr/logs` | ✓ | SSE événements logs |
| POST | `/internal/log` | — | Réception logs CS2 |

## TODO

- [ ] **Enregistrement de démos** — Déclencher `tv_record` / `tv_stoprecord` via RCON au début/fin de match, puis servir le fichier `.dem` en téléchargement (nécessite `tv_enable 1` dans la config serveur et un accès au fichier côté backend)
