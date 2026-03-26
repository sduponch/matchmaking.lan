# matchmaking.lan

Plateforme de matchmaking CS2 pour LAN. Tableau de bord en temps réel avec suivi des matchs, gestion des serveurs, gestion des joueurs/équipes et authentification Steam.

## Composants

| Composant | Stack | Rôle |
|-----------|-------|------|
| `frontend/` | Vue 3, Bootstrap 5, Vite | Dashboard joueurs & admin |
| `backend/` | Go, Gin | API REST, auth Steam, logique match |
| `backend/bot/` | Node.js | Fetch ranks CS2 via Game Coordinator |

## Démarrage rapide

### Prérequis

- Go 1.25+ (gin v1.12.0 requiert 1.25)
- Node.js 18+
- Un serveur CS2 accessible en réseau
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
FRONTEND_URL=https://mm.example.com
BACKEND_URL=https://api.example.com   # URL publique du backend (http:// ou https://)

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

> `BACKEND_URL` est utilisé pour construire l'URL envoyée via `logaddress_add_http`. CS2 supporte `http://` et `https://`.

## Fonctionnalités

### Authentification
- Login via Steam OpenID (popup)
- JWT avec rôle `admin` ou `player`
- Admins définis par Steam64 ID dans `.env`
- Enregistrement automatique du joueur à chaque connexion (`players.json`)

### Profil joueur
- Informations Steam (avatar, niveau, statut)
- Rang CS2 Premier / Compétitif (via bot Game Coordinator)
- Stats Faceit (ELO, niveau, K/D, win rate…)
- Cache avec TTL adaptatif par statut

### Gestion des joueurs (admin)
- Liste de tous les joueurs ayant déjà connecté
- Stats CS2 et Faceit inline avec polling automatique
- Champ équipe synchronisé avec la gestion des équipes

### Gestion des équipes (admin)
- Création / suppression d'équipes
- Ajout / retrait de joueurs par équipe
- Moyennes CS2 Premier et Faceit ELO par équipe
- Synchronisation bidirectionnelle joueur ↔ équipe

### Serveurs LAN
- Découverte automatique par broadcast UDP (protocole A2S)
- Ajout manuel avec mot de passe RCON
- Token unique par serveur pour identification des logs (embarqué dans l'URL)
- Vérification de réception des logs à l'ajout (timeout 5s)
- `last_log_at` en mémoire — détection de serveurs plantés
- Changement de map à distance via RCON
- Comptage joueurs humains (hors bots)

### Suivi de match en temps réel
- Réception des logs CS2 via `logaddress_add_http` (HTTP ou HTTPS)
- Identification du serveur par token dans le path `/internal/log/:token` (fonctionne derrière NAT/proxy)
- Parser de logs DSL avec pattern registry (`patterns.json`)
- Machine d'état par serveur : phase, score CT/T, round, map
- Tableau des scores par équipe : K/D/A, DMG, ADR, argent, MVP
- Avatars Steam dans le tableau
- Mise à jour toutes les 5 secondes

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
    │ logaddress_add_http POST /internal/log/:token
    ▼
HTTPHandler
    │ résolution token → serverAddr (GetAddrByToken)
    │ UpdateLastLog(addr)
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
                        MatchState {phase, map, round, score, players, last_log_at}
```

### Hooks (câblage dans main.go)

| Hook | Rôle |
|------|------|
| `gamelog.OnEvent` | → `match.Apply` |
| `gamelog.OnLog` | → `server.UpdateLastLog` |
| `gamelog.ResolveToken` | → `server.GetAddrByToken` |
| `match.GetLastLogAt` | → `server.GetLastLogAt` |

### Persistance

| Fichier | Contenu |
|---------|---------|
| `servers.json` | `map[addr]{rcon, token}` |
| `players.json` | `map[steamid]{username, avatar, role, team, last_seen}` |
| `teams.json` | `map[id]{name, players[]}` |

`last_log_at` est **en mémoire uniquement** (pas de persistance disque).

### Identification serveur derrière NAT

À l'ajout d'un serveur, le backend envoie via RCON :
```
logaddress_add_http "https://api.example.com/internal/log/<token>"
```
Le token (16 octets hex aléatoires) est persisté dans `servers.json`. Il est extrait du path à chaque POST et résolu en adresse serveur — fonctionne même si tous les serveurs LAN partagent la même IP publique.

### SteamID

Les logs CS2 utilisent le format Steam3 `[U:1:160633]` et les JSON blocks l'`accountid` 32-bit. Tous convertis en Steam64 :

```
steam64 = 76561197960265728 + accountid
```

### Pattern DSL

```json
{ "type": "cs2.kill.headshot", "pattern": "{killer:player} {kpos:pos} killed {victim:player} {vpos:pos} with {weapon:quoted} (headshot)" }
```

Tokens : `player`, `player_nt`, `quoted`, `int`, `word`, `pos`

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/steam` | — | Callback OpenID Steam |
| GET | `/auth/me` | ✓ | Infos joueur connecté |
| GET | `/profile/:steamid` | ✓ | Profil Steam complet |
| GET | `/profile/:steamid/cs2` | ✓ | Stats CS2 |
| GET | `/profile/:steamid/faceit` | ✓ | Stats Faceit |
| GET | `/players` | admin | Liste joueurs enregistrés |
| GET | `/teams` | ✓ | Liste équipes |
| POST | `/teams` | admin | Créer une équipe |
| DELETE | `/teams/:id` | admin | Supprimer une équipe |
| POST | `/teams/:id/players` | admin | Ajouter joueur à l'équipe |
| DELETE | `/teams/:id/players/:steamid` | admin | Retirer joueur de l'équipe |
| GET | `/servers` | ✓ | Liste serveurs LAN |
| POST | `/servers` | admin | Ajouter serveur (RCON + vérif logs) |
| DELETE | `/servers/:addr` | admin | Retirer serveur |
| POST | `/servers/:addr/map` | admin | Changer la map |
| GET | `/servers/:addr/match` | ✓ | État du match + last_log_at |
| GET | `/servers/:addr/logs` | ✓ | SSE événements logs |
| POST | `/internal/log/:token` | — | Réception logs CS2 |

## TODO

- [ ] **Enregistrement de démos** — Déclencher `tv_record` / `tv_stoprecord` via RCON au début/fin de match, puis servir le fichier `.dem` en téléchargement
- [ ] **Tournois** — Bracket, rencontres, gestion des matchs organisés
- [ ] **Matchmaking** — Recherche automatique de parties
