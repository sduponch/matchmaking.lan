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

### API Endpoints — Existants

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
| DELETE | `/servers/:token` | admin | Retirer serveur géré |
| PUT | `/servers/:token/name` | admin | Renommer un serveur (+ hostname RCON) |
| POST | `/servers/:token/map` | admin | Changer la map (RCON changelevel) |
| POST | `/servers/:token/cfg` | admin | Pousser une config via RCON (server_init ou profil) |
| GET | `/servers/:token/match` | ✓ | État du match en cours + last_log_at |
| GET | `/servers/:token/logs` | ✓ | SSE stream des événements de log |
| POST | `/internal/log/:token` | — | Réception logs CS2 (logaddress_add_http, token-based) |
| POST | `/internal/log` | — | Réception logs CS2 (fallback sans token) |
| GET | `/match-profiles` | ✓ | Liste profils de match |
| POST | `/match-profiles` | admin | Créer un profil |
| GET | `/match-profiles/:id` | ✓ | Détail profil (avec tous les CFGs) |
| PUT | `/match-profiles/:id` | admin | Modifier métadonnées profil |
| DELETE | `/match-profiles/:id` | admin | Supprimer profil + CFGs |
| GET | `/match-profiles/:id/cfg/:phase` | ✓ | Contenu CFG d'une phase |
| PUT | `/match-profiles/:id/cfg/:phase` | admin | Écrire CFG d'une phase |
| GET | `/server-init-cfg` | ✓ | Contenu de server_init.cfg |
| PUT | `/server-init-cfg` | admin | Écrire server_init.cfg |

### Persistance

| Fichier | Contenu |
|---------|---------|
| `servers.json` | `map[id]{addr, name, rcon, token}` — ID hex stable, token 16 octets hex aléatoire |
| `players.json` | `map[steamid]{username, avatar, role, team, last_seen}` |
| `teams.json` | `map[id]{name, players[]}` |
| `match_profiles.json` | `map[id]{name, tags[], created_at}` — tags = modes compatibles (vide = tous) |
| `encounters.json` | `map[id]{team1, team2, format, game_mode, side_pick, launch_mode, pick_ban, map_pool, veto_first, decider_side, status, maps[], ...}` |
| `phases.json` | `map[id]{name, type, status, teams[], rounds[], config}` |
| `tournaments.json` | `map[id]{name, status, teams[], stages[]}` |
| `matches/{encounter_id}.json` | Historique round par round d'une rencontre terminée |

`last_log_at` est **en mémoire uniquement** — pas de persistance disque pour éviter les écritures à chaque log.

**Migrations automatiques :**
- `servers.json` ancien format `map[addr]string` → `map[addr]{rcon, token}` → `map[id]{addr, name, rcon, token}`

### Registre joueurs (`backend/internal/registry/`)

- `store.go` : `Player{SteamID, Username, Avatar, Role, Team, LastSeen}`, persisté dans `players.json`
- `Upsert(steamid, username, avatar, role)` — appelé à chaque connexion, préserve le champ `Team` existant
- `SyncRoles(adminIDs)` — appelé au démarrage pour reconcilier les rôles avec `ADMIN_STEAM_IDS`
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

**Store** — `servers.json` est `map[token]*serverEntry` (le token est l'identifiant stable) :
```go
type serverEntry struct {
    Addr  string `json:"addr"`  // "192.168.1.41:27015"
    Name  string `json:"name"`  // nom libre, ex: "Tournoi A - Team1 vs Team2"
    RCON  string `json:"rcon"`
    Token string `json:"token"` // 16 octets hex — ID stable ET token log
}
```

**Migrations automatiques :**
- v1 `map[addr]string` → v2 `map[addr]{rcon, token}` → v3 `map[token]{addr, name, rcon, token}`

- **Découverte A2S** : broadcast UDP `255.255.255.255:27015`, réponse `go-a2s`, déduit joueurs humains = `Players - Bots`
- **Ajout** : teste RCON → `upsertManaged(addr, rcon)` retourne le token → pousse `server_init.cfg` via `sendRCONBatch` → enregistre `ExpectLog` goroutine → envoie `log on` → envoie `logaddress_add_http "BACKEND_URL/internal/log/TOKEN"` → attend confirmation réception (timeout 5s)
- **`last_log_at`** : en mémoire (`map[string]time.Time` keyed by addr), mis à jour à chaque POST reçu
- **`GetAddrByToken(token)`** : résout token → addr — utilisé par `gamelog.ResolveToken`
- **`GetByToken(token)`** : retourne l'entrée complète (addr + rcon + name) — utilisé par encounter/rcon
- **Routes** : toutes les routes `/servers/:token/*` — middleware `resolveServerToken()` dans main.go injecte `"serverAddr"` dans le contexte Gin
- **`PUT /servers/:token/name`** : renomme le serveur + pousse `hostname "name"` via RCON (édition inline dans AdminServersManage.vue)
- **`POST /servers/:token/cfg`** : pousse `server_init.cfg` ou le warmup CFG d'un profil via RCON. Body `{"profile_id": "server_init" | "<id>"}`. Utilisé depuis le dropdown "Configurer" dans AdminServersManage.vue
- **RCON** : `gorcon/rcon`, `sendRCON` (connexion unique) + `sendRCONBatch` (connexion unique pour n commandes)

### Identification serveur derrière NAT

CS2 envoie les logs HTTP depuis l'IP de la passerelle, pas l'IP individuelle des serveurs. Solution : chaque serveur a un token unique (16 octets hex) embarqué dans le path de l'URL `logaddress_add_http`. Le token est résolu en adresse serveur à chaque POST reçu.

```
logaddress_add_http "https://api.example.com/internal/log/<token>"
```

### Hook pattern (évite les imports circulaires)

Les packages `gamelog`, `server`, `match`, `encounter` ne peuvent pas s'importer mutuellement. Les dépendances sont câblées dans `main.go` via des variables de fonction :

```go
gamelog.OnEvent            = match.Apply                 // dispatch événements → machine d'état
gamelog.ResolveToken       = server.GetAddrByToken       // résolution token → addr
gamelog.OnLog              = server.UpdateLastLog        // mise à jour last_log_at
match.GetLastLogAt         = server.GetLastLogAt         // inclus dans GET /servers/:id/match
match.GetEncounterInfo     = encounter.GetByServerID     // sidePick/readyCount/maxRounds pour la machine
match.OnGameOver           = encounter.RecordResult      // résultat map → encounter
match.OnPhaseChange        = ...                         // push CFG + hostname par phase (main.go)
match.OnKnifeChoice        = ...                         // mp_swapteams si nécessaire + live.cfg (main.go)
match.OnFirstPlayerJoin    = ...                         // swap côté si mauvais + teamnames 2s après (main.go)
encounter.OnStart          = match.ExpectWarmup          // arme le flag avant changelevel
encounter.OnComplete       = phase.CheckRoundComplete    // round terminé ?
phase.OnComplete           = tournament.Advance          // avancement tournoi
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
- `cs2.chat` / `cs2.chat.team` — messages chat joueur, utilisés pour les commandes `!ready`, `!ct`, `!t`
- `cs2.map.loading` : `Loading map {map:quoted}` — reset complet de la machine d'état
- `cs2.map.started` : `Started:  {path:quoted}` (deux espaces) — le chemin inclut un suffixe `+prefabs/...` que la machine strip avec `strings.Index(path, "+")`

**JSON blocks :** CS2 envoie des blocs `JSON_BEGIN{...}}JSON_END` contenant les stats de fin de round (`round_stats`) avec les stats CSV par joueur.

**SteamID :** les logs CS2 utilisent le format Steam3 `[U:1:160633]`. Converti en Steam64 via `steam3ToSteam64` : `steam64 = 76561197960265728 + accountid`.

**SSE broker :** `gamelog.Broker` diffuse les événements par serveur. `gamelog.OnEvent` hook pour brancher la machine d'état.

**Hooks exportés :**
- `OnEvent func(*Event)` — déclenché pour chaque événement parsé
- `OnLog func(addr string)` — déclenché à chaque POST reçu (avant parsing), pour `last_log_at`
- `ResolveToken func(token string) (addr string, ok bool)` — résout token → addr serveur

**`ExpectLog(addr, timeout)`** — attend un log entrant pour un serveur donné, utilisé pour vérifier la réception après `logaddress_add_http`.

### Machine d'état (`backend/internal/match/`)

Par serveur, suit : `phase`, `map`, `round`, `score_ct`, `score_t`, `players`, `rounds` (historique).

**Phases :** `idle` → `warmup` → `knife` → `first_half` → `halftime` → `second_half` → `overtime` → `game_over`

**`PlayerStat` :** `name`, `steamid` (Steam64), `team`, `kills`, `deaths`, `assists`, `dmg`, `hsp`, `adr`, `money`, `mvp`

**`RoundSnapshot`** — ajouté à l'historique à chaque fin de round :
```go
type RoundSnapshot struct {
    Number    int           `json:"number"`
    Phase     string        `json:"phase"`      // "first_half" | "second_half" | "overtime"
    WinSide   string        `json:"win_side"`   // "ct" | "t"
    WinReason string        `json:"win_reason"` // "elimination" | "bomb_exploded" | "bomb_defused" | "time"
    Players   []RoundPlayer `json:"players"`
}
```

Stats enrichies à chaque `cs2.round.stats` (JSON block) — `findByAccountID` convertit l'accountid 32-bit en Steam64 pour la lookup directe.

`GET /servers/:id/match` retourne l'état + `last_log_at` (via hook `GetLastLogAt`). Le frontend poll cet endpoint toutes les 5s.

**Cycle de démarrage d'un encounter :**
1. `encounter.Start()` → `OnStart(token)` → `match.ExpectWarmup(addr)` — arme le flag
2. RCON : `changelevel <map>`
3. CS2 : `Loading map "de_ancient"` → reset complet de la machine (état, tous les flags sauf `expectWarmup`)
4. CS2 : `Started:  "de_ancient+prefabs/..."` → si `expectWarmup=true` → `OnPhaseChange("warmup")` + désarme le flag
5. Bot-kick cycles → nouveaux `Started:` mais flag=false → rien poussé (pas de boucle)
6. Premier joueur humain : `Unassigned → CT/T` → `OnFirstPlayerJoin` → swap si mauvais côté + teamnames

**Pourquoi `expectWarmup` et pas `cs2.map.loading`/`cs2.map.started` seuls :**
`Started:  "..."` se déclenche aussi sur les cycles bot-kick (pas seulement sur les changelevel). `cs2.map.loading` idem. Le flag `expectWarmup` armé par `encounter.OnStart` est le seul signal fiable — il ne fire qu'une fois par encounter démarré.

**Flags Machine :**
- `expectWarmup bool` — armé par `ExpectWarmup()`, consommé par `cs2.map.started`
- `halftimeNotified bool` — évite double push mi-temps
- `inKnifeSetup bool` — entre `!ready→knife` et `cs2.match.start` couteau
- `knifeOver bool` — en attente de `!ct`/`!t` après fin du round couteau
- `firstPlayerDone bool` — évite double swap sur re-connect du premier joueur

**Hooks exportés :**
- `GetLastLogAt func(addr string) *time.Time` — câblé vers `server.GetLastLogAt`
- `OnGameOver func(serverAddr string, scoreCT, scoreT int)` — câblé vers `encounter.RecordResult`
- `OnPhaseChange func(serverAddr, phase string)` — câblé dans `main.go` : push CFG + hostname par phase
- `OnKnifeChoice func(serverAddr, winnerSide, chosenSide string)` — câblé dans `main.go` : `mp_swapteams` + live.cfg
- `OnFirstPlayerJoin func(serverAddr, steamID, team string)` — câblé dans `main.go` : swap si mauvais côté + teamnames 2s après
- `GetEncounterInfo func(addr string) (sidePick, readyCount, maxRounds, ok)` — câblé vers `encounter.GetByServerID`
- `ExpectWarmup(addr string)` — fonction publique, arme le flag `expectWarmup`

### Match Config Profiles (`backend/internal/matchconfig/`)

Profils de configuration de match. Chaque profil définit les commandes RCON à envoyer à chaque phase de jeu.

```go
type Profile struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Tags      []string  `json:"tags,omitempty"` // modes compatibles (vide = tous) : "defuse", "wingman", etc.
    CreatedAt time.Time `json:"created_at"`
}
```

`Mode`, `Format`, `KnifeRound`, `Ready` ont été retirés du profil — ces paramètres appartiennent à l'`Encounter`.

**CFG files** stockés dans `backend/configs/{profile_id}/{phase}.cfg`. Format : une commande RCON par ligne (commentaires `//` ignorés, inline `//` strippés). À chaque transition de phase, chaque ligne est envoyée via RCON au serveur via `sendRCONBatch`.

**`server_init.cfg`** stocké dans `backend/configs/server_init.cfg`. Poussé automatiquement à l'ajout de chaque serveur. Contient par défaut : mode Deathmatch (`game_type 1 / game_mode 2`), hostname `"Warmup deathmatch"`, respawn, armure gratuite, config GOTV.

**Phases supportées :** `warmup`, `knife`, `live`, `halftime`, `game_over`

**`ParseCFG(content)`** : skippe lignes vides et `//` comments, strippes commentaires inline.

**`GetServerInitCommands()`** : retourne les commandes parsées de `server_init.cfg`.

**`GetProfileWarmupCommands(profileID)`** : retourne les commandes parsées du warmup CFG d'un profil — utilisé par `HandlePushCFG`.

**Seeding** (`init()`) : `seedServerInitCFG()` et `seedDefaultProfile()` — créent les fichiers par défaut si absents.

**Déclenchements automatiques :**
| Événement | Action |
|---|---|
| `cs2.map.started` (+ `expectWarmup=true`) | push game_mode + warmup.cfg + hostname |
| `cs2.warmup.end` | `tv_record "enc_<id>"` |
| `cs2.chat "!ready"` (tous ready, `side_pick=knife`) | push knife.cfg + hostname |
| `cs2.chat "!ready"` (tous ready, side fixe) | push live.cfg + hostname (first_half) |
| `cs2.chat "!ct"/"!t"` (couteau terminé) | `mp_swapteams` si nécessaire + push live.cfg + hostname |
| score == maxRounds/2 | push halftime.cfg + hostname |
| `cs2.match.start` (après halftime) | push hostname (second_half) |
| `cs2.game.over` | push game_over.cfg + hostname |

**Demo recording :** `tv_record "enc_<id>"` via RCON déclenché par `cs2.warmup.end`.

**API :**
```
GET    /match-profiles
POST   /match-profiles
GET    /match-profiles/:id
PUT    /match-profiles/:id
DELETE /match-profiles/:id
GET    /match-profiles/:id/cfg/:phase   — contenu du fichier cfg
PUT    /match-profiles/:id/cfg/:phase   — mise à jour du fichier cfg
GET    /server-init-cfg
PUT    /server-init-cfg
```

### Encounter — Rencontre (`backend/internal/encounter/`)

Unité de base : Team A vs Team B, bo1/bo3/bo5. Peut exister de façon standalone (match amical) ou dans le contexte d'une phase de tournoi.

```go
type Encounter struct {
    ID          string     `json:"id"`
    Team1       string     `json:"team1"`                  // team ID
    Team2       string     `json:"team2"`
    Format      string     `json:"format"`                 // "bo1" | "bo3" | "bo5"
    GameMode    string     `json:"game_mode"`              // "defuse"|"casual"|"wingman"|"retakes"|"hostage"|"armsrace"|"deathmatch"
    SidePick    string     `json:"side_pick"`              // "knife" | "ct" | "t"  (ignoré si pick_ban=true)
    LaunchMode  string     `json:"launch_mode"`            // "manual" | "scheduled" | "ready"
    ReadyCount  int        `json:"ready_count,omitempty"`  // joueurs requis si launch_mode="ready"
    PickBan     bool       `json:"pick_ban"`               // sélection de carte par pick & ban
    MapPool     []string   `json:"map_pool,omitempty"`     // cartes éligibles au pick & ban
    VetoFirst   string     `json:"veto_first,omitempty"`   // "seed" | "toss" | "chifoumi"
    DeciderSide string     `json:"decider_side,omitempty"` // "pickban" | "toss" | "knife" | "vote"
    Status      string     `json:"status"`                 // "scheduled" | "live" | "completed"
    ServerID    string     `json:"server_id,omitempty"`
    ProfileID   string     `json:"profile_id,omitempty"`
    Hostname    string     `json:"hostname,omitempty"`     // base hostname sans suffixe de phase
    Maps        []GameMap  `json:"maps"`
    Winner      string     `json:"winner,omitempty"`       // "team1" | "team2"
    DemoStatus  string     `json:"demo_status"`            // "none" | "recording"
    CreatedAt   time.Time  `json:"created_at"`
    ScheduledAt *time.Time `json:"scheduled_at,omitempty"` // si launch_mode="scheduled"
    StartedAt   *time.Time `json:"started_at,omitempty"`
    EndedAt     *time.Time `json:"ended_at,omitempty"`
}
type GameMap struct {
    Number int    `json:"number"`
    Map    string `json:"map"`
    Score1 int    `json:"score1"`
    Score2 int    `json:"score2"`
    Winner string `json:"winner,omitempty"`
    Status string `json:"status"` // "pending" | "live" | "completed"
}
```

**`GameModeCommands(mode string) []string`** — retourne les commandes RCON `game_type`/`game_mode` pour un mode donné. Défuse/hostage = compétitif (`0/1` + `mp_retakes 0`), retakes = `0/1` + `mp_retakes 1`, wingman = `0/2`, casual = `0/0`, armsrace = `1/0`, deathmatch = `1/2`.

- `RecordResult(serverToken, scoreCT, scoreT)` — hook appelé par `match.OnGameOver`, met à jour la map courante, calcule le gagnant bo1/3/5, passe `status` à `completed` si toutes les maps jouées
- `OnComplete func(enc *Encounter)` — hook câblé vers `phase.CheckRoundComplete`
- `OnStart func(serverToken string)` — hook appelé par `Start()` juste avant d'envoyer le `changelevel` RCON ; câblé vers `match.ExpectWarmup` pour armer le flag avant que CS2 commence le chargement
- `HandleStart` : appelle `OnStart` → push `GameModeCommands` + warmup CFG + hostname + `changelevel <map>` (en dernier). Hostname inclut noms des équipes + labels CT/T + phase (ex: "Nexen (CT) vs Hello!World (T) - Warmup")
- `HandleReopen` : remet `status=scheduled`, vide winner/server/dates/scores
- `OnPhaseChange("warmup")` (via `match.OnPhaseChange` hook) : re-pousse game mode + warmup CFG + hostname avec labels CT/T après chaque changelevel (CS2 reset les ConVars)
- `OnPhaseChange("warmup_end")` : déclenche `tv_record "enc_<id>"`
- `buildHostname(enc, ctIsTeam1 bool, phase string)` (dans main.go) : construit le hostname complet avec les noms d'équipe, labels CT/T, et suffixe de phase. Labels phase : warmup="Warmup", first_half="1ère Mi-temps" (ou "Live" pour deathmatch/armsrace/casual), second_half="2ème Mi-temps", halftime="Mi-temps", overtime="Prolongation", knife="Couteaux", game_over="Terminé"
- `logaddress_delall_http` envoyé avant `logaddress_add_http` à l'ajout serveur (évite l'accumulation)

**API :**
```
GET    /encounters
POST   /encounters
GET    /encounters/:id
PUT    /encounters/:id
POST   /encounters/:id/start     — assigne serveur + profil, push warmup cfg + changelevel
POST   /encounters/:id/reopen    — réinitialise en "scheduled"
POST   /encounters/:id/result    — override manuel résultat d'une map (admin)
DELETE /encounters/:id
```

### Phase de tournoi (`backend/internal/phase/`)

Contient des rounds qui contiennent des encounters. Types : `groups`, `swiss`, `upper_bracket`, `lower_bracket`, `single_elimination`.

```go
type Phase struct {
    ID     string      `json:"id"`
    Name   string      `json:"name"`
    Type   string      `json:"type"`
    Status string      `json:"status"` // draft | active | completed
    Teams  []string    `json:"teams"`  // IDs seedés dans cette phase
    Rounds []Round     `json:"rounds"`
    Config PhaseConfig `json:"config"`
}
type PhaseConfig struct {
    MatchFormat         string `json:"match_format"`    // bo1 | bo3 | bo5
    ProfileID           string `json:"profile_id"`
    SeedingSource       string `json:"seeding_source"`  // "faceit" | "cs2" | "manual" | "random"
    // Poules
    NumGroups           int    `json:"num_groups,omitempty"`
    AdvanceCount        int    `json:"advance_count,omitempty"`
    // Suisse
    NumRounds           int    `json:"num_rounds,omitempty"`
    WinCondition        int    `json:"win_condition,omitempty"`
    // Bracket
    GrandFinalAdvantage bool   `json:"grand_final_advantage,omitempty"`
}
```

**Seeding** (`POST /phases/:id/seed`) :
- `faceit` → moyenne ELO Faceit des joueurs de l'équipe (via stats cache)
- `cs2` → moyenne rating CS2 Premier
- `manual` → ordre fourni par l'admin
- `random` → tirage aléatoire

**Génération des rounds** (`POST /phases/:id/rounds/generate`) :
- **Poules** : round-robin dans chaque groupe, tous les encounters générés en une fois
- **Suisse** : génère la ronde suivante selon le record W/L actuel (appariement Monrad)
- **Brackets** : générés depuis les qualifiés de la phase précédente, arbre single ou double élimination

**Standings** : calculés dynamiquement depuis les encounters complétés (victoires, défaites, diff maps, diff rounds).

**`OnComplete func(phase *Phase)`** — hook câblé vers `tournament.Advance`

**API :**
```
GET    /phases
POST   /phases
GET    /phases/:id
PUT    /phases/:id
POST   /phases/:id/seed
POST   /phases/:id/rounds/generate
DELETE /phases/:id
```

### Tournoi (`backend/internal/tournament/`)

Orchestre les phases et gère l'avancement automatique.

```go
type Tournament struct {
    ID        string            `json:"id"`
    Name      string            `json:"name"`
    Status    string            `json:"status"` // draft | active | completed
    Teams     []string          `json:"teams"`
    Stages    []TournamentStage `json:"stages"` // phases ordonnées
    CreatedAt time.Time         `json:"created_at"`
}
type TournamentStage struct {
    Order        int    `json:"order"`
    PhaseID      string `json:"phase_id"`
    AdvanceCount int    `json:"advance_count"` // N équipes qualifiées pour l'étape suivante
}
```

**Avancement automatique** (`tournament.Advance`) : quand une phase est `completed`, prend les `advance_count` premières équipes du classement, les injecte dans la phase suivante avec leur seed, et active cette phase.

**API :**
```
GET    /tournaments
POST   /tournaments
GET    /tournaments/:id     vue complète (phases + standings + bracket)
PUT    /tournaments/:id
POST   /tournaments/:id/start
DELETE /tournaments/:id
```

### Frontend (`frontend/src/`)

**Stores Pinia :**
- `auth.ts` — token JWT, user (steamid, username, avatar, role), `init()` / `setToken()` / `logout()`
- `app-option.ts` — état UI layout (sidebar, header, thème…)

**Vues admin (existantes) :**
- `AdminPlayers.vue` — tableau joueurs avec avatar/rôle/équipe/stats CS2+Faceit/dernière connexion, poll `retrieving`
- `AdminMatchmakingTeams.vue` — accordion : liste équipes avec moyennes CS2/Faceit, composition, ajout/retrait joueurs, création équipe (modal)
- `AdminServersManage.vue` — tableau serveurs avec colonne "Dernier log" + nom éditable (inline) + dropdown "Configurer" (pousse server_init ou warmup d'un profil via RCON)
- `AdminServerSetup.vue` — ajout serveur avec vérification réception logs
- `AdminMatchConfigs.vue` — card server_init.cfg (textarea monospace + save) + accordion profils de match (onglets par phase, CRUD profil, modal création) + tag badges par mode de jeu
- `AdminEncounters.vue` — liste rencontres (accordion détail maps + infos) ; création modale structurée en 4 sections :
  - **Choix de la carte** : Manuel (selects filtrés par mode) ou Pick & Ban (pool officiel + serveurs, cases à cocher)
  - **Choix du côté de départ** : si Pick & Ban → qui commence le veto (Seed/Aléatoire/Challenge) + camp carte décisive (Pick&Ban/Aléatoire/Knife/Vote) ; si Manuel → boutons Couteaux/CT/T
  - **Mode de lancement** : Manuel / Planifié (datetime-local) / Ready (compteur joueurs)
  - Démarrage (modal serveur + profil + label événement), override résultat, réinitialisation

**Vues admin (à créer) :**
- `AdminPhase.vue` — gestion phase : seeding, génération rounds, standings
- `AdminTournament.vue` — vue tournoi globale : bracket visualisation, avancement

**Sidebar admin :**
- Serveurs : Configurer (`/admin/server/setup`), Counter-Strike 2 (`/admin/servers`), Profils de match (`/admin/matchmaking/match-configs`)
- Matchmaking : Joueurs, Équipes, Rencontres (`fa-shield-halved`), Tournois

**Vues joueur (existantes) :**
- `PageLogin.vue` — bouton login Steam (popup OpenID)
- `PageAuthDone.vue` — récepteur postMessage après auth
- `Profile.vue` — profil joueur, poll `/cs2` et `/faceit` séparément tant que `retrieving`
- `Home.vue` — liste serveurs LAN, état match en temps réel, contrôles admin

**Config :** `src/config/matchmaking.ts` — `api.baseUrl`, `admins.steamIds`

**Ne jamais modifier** `src/env.d.ts`.

### Layout system
`App.vue` conditionne Header + Sidebar + `<RouterView>` + Footer via `appOptionStore`. `@` alias → `src/`.

### Styling
Bootstrap 5, SCSS dans `src/scss/`. Dark mode par défaut (`data-bs-theme="dark"`).

## Plan d'implémentation

```
✅ Auth Steam + JWT
✅ Registre joueurs (players.json) + sync rôles au démarrage (SyncRoles)
✅ Gestion équipes (teams.json)
✅ Stats CS2 + Faceit (cache)
✅ Serveurs LAN (A2S + RCON + logaddress token)
✅ Machine d'état match (gamelog → phases → stats)
✅ Étape 1 — Refacto server : token comme ID stable + Name éditable (PUT /servers/:token/name)
✅ Étape 2 — Match Config Profiles + CFG editor
       Backend : matchconfig/ (store, cfg, handler), server_init.cfg (deathmatch par défaut),
                 sendRCONBatch, POST /servers/:token/cfg (dropdown "Configurer")
       Frontend : AdminMatchConfigs.vue (server_init + accordion profils + éditeur CFG par phase)
                  sidebar Serveurs → Profils de match

🔄 Étape 3 — Encounter (rencontre standalone + intégration CS2 + démos)
       ✅ Backend : struct Encounter complet (game_mode, side_pick, launch_mode, pick_ban, map_pool, veto_first, decider_side)
       ✅ Backend : CRUD + start + reopen + setResult ; GameModeCommands ; OnPhaseChange hook
       ✅ Backend : changelevel + re-push warmup CFG après map change ; tv_record à warmup_end
       ✅ Backend : hostname avec noms d'équipes + labels CT/T par phase
       ✅ Backend : expectWarmup flag (évite la boucle bot-kick) ; OnFirstPlayerJoin (swap côté + teamnames)
       ✅ Backend : déclenchement automatique des phases (knife, first_half, halftime, second_half via machine d'état)
       ✅ Frontend : AdminEncounters.vue (liste, création modale 4 sections, démarrage, override résultat)
       ⬜ Logique pick & ban interactive (interface joueurs + résolution veto)
⬜ Étape 4 — Round history (extension match machine + persistance)
⬜ Étape 5 — Phase : poules avec seeding Faceit/CS2
⬜ Étape 6 — Phase : ronde suisse
⬜ Étape 7 — Phase : brackets upper/lower (double élimination)
⬜ Étape 8 — Tournament : orchestration + avancement automatique
⬜ Étape 9 — Frontend : vues par couche (encounters → phases → tournoi)
```

## Notes techniques importantes

- **`go build ./...`** est la seule façon fiable de vérifier le code Go — gopls affiche de faux positifs car `go.work` a été supprimé (gin v1.12.0 requiert go 1.25.0, système a go 1.24.4).
- **`go.work` supprimé** — ne pas recréer, gopls ne fonctionne pas correctement dans ce contexte.
- **`logaddress_add_http`** — n'accepte qu'un seul paramètre URI. Le token doit être dans le path, pas en second argument. CS2 supporte `http://` et `https://`.
- **Imports circulaires** — `gamelog`, `server`, `match`, `encounter`, `phase`, `tournament` ne peuvent pas s'importer entre eux. Utiliser le pattern hook via `main.go`.
- **Servers référencés par ID** — les encounters, phases et tournaments utilisent `server_id` (jamais l'addr directement). `server.GetByID(id)` retourne l'addr + rcon nécessaires pour RCON.
- **`expectWarmup` flag** — seul signal fiable pour déclencher la warmup CFG après un `changelevel`. `cs2.map.started` et `cs2.map.loading` se déclenchent aussi sur les cycles bot-kick. Le flag est armé par `encounter.OnStart` → `match.ExpectWarmup()` juste avant le changelevel, et consommé sur le premier `cs2.map.started`.
- **`mp_swapteams` side effect** — déclenche un `Restart_Round_(1_second)` côté CS2, ce qui réinitialise les noms d'équipes. Les `mp_teamname_1`/`mp_teamname_2` doivent être poussés 2s après dans une goroutine séparée.
