# Documentation Backend — matchmaking.lan

## Architecture générale

Serveur Go (Gin) sans base de données externe. Toutes les données sont persistées dans des fichiers JSON. Communication frontend via REST + JWT + SSE. Un subprocess Node.js gère les ranks CS2 via le Game Coordinator Steam.

---

## `auth` — Authentification Steam + JWT

### Principe
Flux OpenID Steam en popup côté client → callback Go → validation signature → génération JWT.

### `jwt.go`
| Élément | Description |
|---------|-------------|
| `Claims` | Struct JWT : `SteamID`, `Username`, `AvatarURL`, `Role` (`admin`/`player`) |
| `GenerateJWT(steamID, username, avatarURL, role)` | Crée et signe un JWT (durée configurable via `JWT_EXPIRY`) |
| `ParseJWT(tokenStr)` | Valide et décode un JWT, retourne les claims ou une erreur |

### `steam.go`
| Élément | Description |
|---------|-------------|
| `HandleCallback(c)` | Handler Gin pour le retour OpenID. Valide la signature, extrait le SteamID, récupère le profil Steam, upsert le registre, émet le JWT, redirige vers le frontend |
| `validateOpenID(r)` | Revalide la signature OpenID auprès des serveurs Steam |
| `extractSteamID(claimedID)` | Extrait l'ID numérique depuis l'URL `claimed_id` OpenID par regex |
| `fetchSteamProfile(steamID)` | Appel API Steam pour récupérer username et avatar |

---

## `bot` (Go) — Gestionnaire du subprocess Node.js

### Principe
Lance et surveille le processus Node.js (`backend/bot/index.js`). Redémarre automatiquement en cas de crash. Expose une interface HTTP interne pour récupérer les ranks CS2.

### `manager.go`
| Élément | Description |
|---------|-------------|
| `RankInfo` | `PremierRating`, `CompetitiveRank`, `CompetitiveWins` |
| `Manager` | Contient le port, le chemin du bot et le `*exec.Cmd` actif |
| `NewManager(port)` | Crée le manager, calcule le chemin vers `bot/` |
| `(m) Start(ctx)` | Lance le subprocess en arrière-plan |
| `(m) run(ctx)` | Boucle interne : spawn Node.js, relance après 5s en cas d'erreur |
| `(m) GetRank(steamID)` | `GET /rank/{steamID}` vers le bot, timeout 12s |
| `(m) GetInfo()` | `GET /info` vers le bot, retourne le SteamID du bot |

---

## `bot/index.js` — Bot Steam CS2 (Node.js)

### Principe
Se connecte à Steam et au Game Coordinator CS2 via `globaloffensive`. Expose un serveur HTTP local. Si le GC ne répond pas (profil privé), envoie une demande d'ami et retourne `pending_invite`.

| Élément | Description |
|---------|-------------|
| `getRank(steamid)` | Récupère le profil GC avec cache 5min. Timeout 10s → demande d'ami si nécessaire |
| `extractRank(profile, rankTypeId)` | Extrait le rank par type : `6`=Compétitif, `7`=Wingman, `11`=Premier |
| `GET /rank/:steamid` | Retourne `{premier_rating, competitive_rank, competitive_wins}` |
| `GET /info` | Retourne le SteamID du bot |
| `friendRelationship` | Accepte automatiquement les demandes d'amis entrantes |

---

## `config` — Configuration

### Principe
Charge les variables d'environnement depuis `.env` au démarrage. Variables requises : `STEAM_API_KEY`, `JWT_SECRET`.

| Élément | Description |
|---------|-------------|
| `Config` | `Port`, `FrontendURL`, `BackendURL`, `SteamAPIKey`, `AdminSteamIDs`, `JWTSecret`, `JWTExpiry`, `BotPort`, `FaceitAPIKey` |
| `C` | Instance globale accessible par tous les packages |
| `Load()` | Lit `.env` + variables d'environnement, parse les durées, construit la map des admins |

---

## `registry` — Registre joueurs

### Principe
Enregistre tous les joueurs qui se connectent via Steam OpenID. Persisté dans `players.json`. Source de vérité pour les rôles et les équipes.

### `store.go`
| Élément | Description |
|---------|-------------|
| `Player` | `SteamID`, `Username`, `Avatar`, `Role`, `Team`, `LastSeen` |
| `Upsert(steamid, username, avatar, role)` | Crée ou met à jour un joueur à chaque connexion. Préserve le champ `Team` existant |
| `SyncRoles(adminIDs)` | Appelé au démarrage — reconcilie les rôles avec `ADMIN_STEAM_IDS` |
| `SetTeam(steamid, team)` | Mise à jour bidirectionnelle depuis le package `teams` |
| `List()` | Retourne tous les joueurs triés par `last_seen` décroissant |

### `handler.go`
| Élément | Description |
|---------|-------------|
| `HandleList()` | `GET /players` — admin uniquement |

---

## `teams` — Gestion des équipes

### Principe
Équipes avec liste de joueurs. Synchronisation bidirectionnelle avec le registre joueurs via `registry.SetTeam()`.

### `store.go`
| Élément | Description |
|---------|-------------|
| `Team` | `ID`, `Name`, `Players` ([]SteamID), `CreatedAt` |
| `Create(name)` | Crée une équipe avec ID hex aléatoire |
| `Delete(id)` | Supprime l'équipe et efface le champ équipe de tous ses membres |
| `AddPlayer(teamID, steamid)` | Ajoute le joueur (idempotent) + met à jour le registre |
| `RemovePlayer(teamID, steamid)` | Retire le joueur + met à jour le registre |
| `List()`, `Get(id)` | Lecture |

### `handler.go`
5 handlers : `HandleList`, `HandleCreate`, `HandleDelete`, `HandleAddPlayer`, `HandleRemovePlayer`

---

## `server` — Serveurs CS2

### Principe
Découverte UDP (A2S broadcast), gestion RCON, réception des logs CS2 par token. Chaque serveur a un token hex 16 octets stable qui sert à la fois d'identifiant et de chemin dans l'URL `logaddress_add_http`.

### `store.go`
| Élément | Description |
|---------|-------------|
| `serverEntry` | `Addr`, `Name`, `RCON`, `Token`, `Maps` |
| `GetAddrByToken(token)` | Résolution token → addr (utilisée par `gamelog.ResolveToken`) |
| `GetAddrRCON(token)` | Retourne addr + mot de passe RCON (pour RCON batch) |
| `GetTokenByAddr(addr)` | Résolution inverse addr → token |
| `UpdateLastLog(addr)` | Enregistre en mémoire le timestamp du dernier log reçu |
| `GetLastLogAt(addr)` | Retourne ce timestamp (exposé dans `GET /servers/:token/match`) |

### `rcon.go`
| Élément | Description |
|---------|-------------|
| `sendRCON(addr, password, command)` | Une commande RCON sur une connexion unique |
| `SendRCONBatch(addr, password, commands)` | N commandes sur une connexion unique |
| `FetchMaps(addr, password)` | Envoie `maps *` via RCON, parse les maps jouables |
| `HandleAdd()` | Teste RCON → upsert → push `server_init.cfg` → enregistre `logaddress_add_http` → attend confirmation log (timeout 5s) |
| `HandleList()` | Découverte A2S broadcast + serveurs gérés, query A2S chacun |
| `HandleSetName()` | Renomme + pousse `hostname "name"` via RCON |
| `HandlePushCFG()` | Pousse `server_init.cfg` ou le warmup d'un profil via RCON |
| `HandleChangeMap()` | `changelevel <map>` via RCON |

### `discovery.go`
| Élément | Description |
|---------|-------------|
| `discoverLAN(timeout)` | Broadcast UDP `255.255.255.255:27015`, collecte les adresses qui répondent |

---

## `gamelog` — Réception et dispatch des logs CS2

### Principe
CS2 envoie les logs par HTTP POST (`logaddress_add_http`). Chaque ligne est parsée contre un registre de patterns DSL. Les événements sont diffusés via SSE aux clients connectés et dispatchés vers la machine d'état via `OnEvent`.

### `listener.go` + `handler.go`
| Élément | Description |
|---------|-------------|
| `HTTPHandler(w, r)` | Handler POST — résout le token, met à jour `last_log_at`, parse ligne par ligne (ou JSON blocks), dispatch `OnEvent` + broker |
| `Broker` | Broker SSE par serveur. `Subscribe(addr)` / `Unsubscribe(addr, ch)` |
| `HandleSSE()` | `GET /servers/{token}/logs` — stream SSE des événements en temps réel |
| `ExpectLog(addr, timeout)` | Bloque jusqu'à recevoir un log de ce serveur (utilisé à l'ajout serveur pour vérifier la réception) |

### Hooks exportés (câblés dans `main.go`)
| Hook | Câblé vers |
|------|-----------|
| `OnEvent func(*Event)` | `match.Apply` — dispatch vers la machine d'état |
| `OnLog func(addr)` | `server.UpdateLastLog` — mise à jour `last_log_at` |
| `ResolveToken func(token) (addr, bool)` | `server.GetAddrByToken` — résolution token → addr |

### `patterns.go`
| Élément | Description |
|---------|-------------|
| `PatternDef` | `Type` (ex: `cs2.kill`), `Pattern` DSL, regex compilée |
| `Registry` | Slice de tous les patterns compilés depuis `patterns.json` |
| Tokens DSL | `{name:player}`, `{name:quoted}`, `{name:int}`, `{name:word}`, `{name:pos}`, `{name:player_nt}` |

### `jsonblock.go`
| Élément | Description |
|---------|-------------|
| `parseJSONBlock(...)` | Parse les blocs `JSON_BEGIN{...}JSON_END` (stats fin de round). Corrige les virgules manquantes. Retourne un `Event` de type `cs2.round.stats` avec payload `RoundStats` |

### Types d'événements (`event.go`)
| Élément | Description |
|---------|-------------|
| `Event` | `Type`, `Server`, `At`, `Fields` (captures nommées), `Extra` (payload complexe) |
| `RoundStats` | `Round`, `ScoreCT`, `ScoreT`, `Map`, `Players` map |
| `RoundPlayer` | Stats par joueur dans un round : kills, deaths, assists, damage, HSP, ADR, MVP, money |

---

## `match` — Machine d'état par serveur

### Principe
Une `Machine` par serveur, créée à la demande. Reçoit les événements gamelog et met à jour l'état du match (phase, score, stats joueurs). Persiste les rounds en mémoire.

### `machine.go`
| Élément | Description |
|---------|-------------|
| `Machine` | État courant + mutex + flag `warmupNotified` |
| `(m) Apply(e *Event)` | Traite ~20 types d'événements. Exemples : `game.commencing` → reset ; `warmup.start` → phase warmup + `OnPhaseChange` ; `round.end` → score ; `cs2.round.stats` → enrichit les stats joueurs ; `game.over` → `OnGameOver` |
| `(m) findByAccountID(accountID)` | Convertit un accountID 32-bit CS2 en Steam64 pour lookup |
| `steam3ToSteam64(steamid)` | Convertit `[U:1:160633]` → `76561197960265728 + accountID` |

### `manager.go`
| Élément | Description |
|---------|-------------|
| `Get(serverAddr)` | Retourne ou crée la machine pour un serveur |
| `Apply(e *Event)` | Point d'entrée depuis `gamelog.OnEvent` |
| `HandleGetState()` | `GET /servers/{token}/match` — retourne état courant + `last_log_at` |

### Hooks exportés
| Hook | Câblé vers |
|------|-----------|
| `GetLastLogAt func(addr)` | `server.GetLastLogAt` |
| `OnGameOver func(serverToken, scoreCT, scoreT)` | `encounter.RecordResult` |
| `OnPhaseChange func(serverAddr, phase)` | Warmup → re-push game mode + CFG + hostname ; `warmup_end` → `tv_record` ; autres → hostname |

---

## `encounter` — Rencontres

### Principe
Unité de base : Team A vs Team B, bo1/bo3/bo5. Peut être standalone ou dans un tournoi. Gère le cycle de vie complet : création → démarrage → suivi des maps → résultat.

### `store.go` — Champs clés de `Encounter`
| Champ | Description |
|-------|-------------|
| `Format` | `bo1`/`bo3`/`bo5` |
| `GameMode` | `defuse`/`casual`/`wingman`/`retakes`/`hostage`/`armsrace`/`deathmatch` |
| `SidePick` | `knife`/`ct`/`t` (ignoré si pick & ban actif) |
| `LaunchMode` | `manual`/`scheduled`/`ready` |
| `PickBan` + `MapPool` | Pool de cartes pour le veto |
| `VetoFirst` | Qui commence le veto : `seed`/`toss`/`chifoumi` |
| `DeciderSide` | Côté carte décisive : `pickban`/`toss`/`knife`/`vote` |
| `MaxRounds` | `mp_maxrounds` total (ex: 24 = MR12) |
| `Prac` | `mp_ignore_round_win_conditions 1` — les deux équipes jouent tous les rounds |
| `Overtime` + `OTStartMoney` | Prolongations + argent de départ OT |
| `TacticalTimeouts` / `TacticalTimeoutTime` / `TacticalTimeoutsOT` | Pauses tactiques en temps réglementaire et en OT |

### Fonctions principales
| Fonction | Description |
|----------|-------------|
| `Create(enc)` | Crée avec ID généré, initialise les `GameMap` selon le format |
| `Start(id, serverID, profileID, hostname)` | Marque live, assigne serveur/profil, marque la 1ère map live |
| `RecordResult(serverToken, scoreCT, scoreT)` | Appelé par `match.OnGameOver` — enregistre le score, calcule le gagnant de la série, appelle `OnComplete` |
| `SetResult(encID, mapNumber, score1, score2)` | Override manuel admin |
| `HandleReopen()` | Remet `status=scheduled`, vide winner/server/dates/scores |
| `GameModeCommands(mode)` | Retourne les commandes RCON `game_type`/`game_mode` pour un mode donné |

### Hook exporté
| Hook | Câblé vers |
|------|-----------|
| `OnComplete func(*Encounter)` | `phase.CheckRoundComplete` (Étape 5+) |

---

## `matchconfig` — Profils de match et CFG

### Principe
Profils nommés avec tags de modes compatibles. Chaque profil a des fichiers CFG par phase (warmup, knife, live, halftime, game_over) stockés dans `backend/configs/{profile_id}/{phase}.cfg`. Un `server_init.cfg` global est poussé à l'ajout de chaque serveur.

### `store.go`
| Élément | Description |
|---------|-------------|
| `Profile` | `ID`, `Name`, `Tags` (modes compatibles, vide = tous), `CreatedAt` |
| `Phases` | `["warmup", "knife", "live", "halftime", "game_over"]` |
| `seedDefaultProfile()` | Crée un profil "5v5 Compétitif" par défaut si absent |

### `cfg.go`
| Élément | Description |
|---------|-------------|
| `GetCFG(profileID, phase)` | Lit le fichier CFG, retourne `""` si absent |
| `SetCFG(profileID, phase, content)` | Écrit le fichier CFG |
| `ParseCFG(content)` | Parse les lignes : ignore vides et commentaires `//`, strippe les commentaires inline |
| `GetServerInitCommands()` | Retourne les commandes parsées de `server_init.cfg` |
| `GetProfileWarmupCommands(profileID)` | Retourne les commandes parsées du warmup CFG |
| `seedServerInitCFG()` | Crée un `server_init.cfg` par défaut (deathmatch, respawn, armure, GOTV) |

---

## `mappoolconfig` — Pool de cartes officiel

### Principe
Stocke le pool officiel par préfixe de mode (`de_`, `cs_`, `ar_`, `dm_`) dans `map_pool.json`. Seedé avec les cartes CS2 actuelles au premier démarrage. Modifiable par les admins via l'API et la page `/admin/cs2`.

| Élément | Description |
|---------|-------------|
| `Pool` | `map[string][]string` — préfixe → liste de cartes |
| `Get()` | Retourne une copie complète du pool |
| `Set(p)` | Remplace et persiste dans `map_pool.json` |
| `ForPrefix(prefix)` | Retourne les cartes pour un préfixe donné |
| `HandleGet()` | `GET /map-pool` |
| `HandleSet()` | `PUT /map-pool` — admin |

---

## `player` — Profils joueurs agrégés

### Principe
Agrège les données Steam, CS2 (via bot) et Faceit avec un cache par statut. Les fetches sont déclenchées en arrière-plan pour ne pas bloquer la réponse initiale. Le frontend poll les endpoints `/cs2` et `/faceit` tant que le statut est `retrieving`.

### Statuts et TTL (`stats_cache.go`)
| Statut CS2 | TTL | Signification |
|-----------|-----|---------------|
| `retrieving` | — | Fetch en cours |
| `ready` | 5 min | Données disponibles |
| `pending_invite` | 2 min | Profil privé, demande d'ami envoyée |
| `unavailable` | 30s | Erreur GC ou bot indisponible |

| Statut Faceit | TTL | Signification |
|--------------|-----|---------------|
| `retrieving` | — | Fetch en cours |
| `ready` | 5 min | Données disponibles |
| `not_found` | 10 min | Compte Faceit introuvable |
| `unavailable` | 30s | Erreur API |

### `handler.go`
| Élément | Description |
|---------|-------------|
| `HandleGetProfile(bm)` | `GET /profile/{steamid}` — profil Steam complet + statuts CS2/Faceit |
| `HandleGetCS2(bm)` | `GET /profile/{steamid}/cs2` — poll par le frontend tant que `retrieving` |
| `HandleGetFaceit()` | `GET /profile/{steamid}/faceit` — poll par le frontend tant que `retrieving` |

---

## `faceit` — Client API Faceit

| Élément | Description |
|---------|-------------|
| `GetPlayerBySteamID(steamID)` | Recherche par SteamID → récupère profil + stats lifetime CS2 (fallback CSGO) |
| `Stats` | `PlayerInfo` (ELO, level, Faceit URL) + `PlayerStats` (matches, wins, K/D, HS%) |

---

## `cmd/server/main.go` — Point d'entrée

### Câblage des hooks (évite les imports circulaires)

Les packages `gamelog`, `server`, `match`, `encounter` ne peuvent pas s'importer mutuellement. Les dépendances sont câblées dans `main.go` via des variables de fonction :

```
gamelog.OnEvent       → match.Apply
gamelog.ResolveToken  → server.GetAddrByToken
gamelog.OnLog         → server.UpdateLastLog
match.GetLastLogAt    → server.GetLastLogAt
match.OnGameOver      → encounter.RecordResult
match.OnPhaseChange   → push RCON (game mode + CFG + hostname + tv_record)
```

### Middlewares
| Middleware | Description |
|-----------|-------------|
| `requireAuth()` | Valide le JWT dans le header `Authorization: Bearer <token>` |
| `requireAdmin()` | Vérifie `claims.Role == "admin"` |
| `resolveServerToken()` | Résout le token d'URL en adresse serveur, injecte `"serverAddr"` dans le contexte Gin |
| `corsMiddleware()` | Allow-Origin depuis `config.C.FrontendURL` |

---

## Fichiers de persistance

| Fichier | Contenu | Package responsable |
|---------|---------|-------------------|
| `players.json` | Registre joueurs | `registry` |
| `teams.json` | Équipes + compositions | `teams` |
| `servers.json` | Serveurs gérés (token, addr, rcon, name) | `server` |
| `encounters.json` | Rencontres (toutes phases de vie) | `encounter` |
| `match_profiles.json` | Métadonnées profils de match | `matchconfig` |
| `map_pool.json` | Pool officiel par préfixe | `mappoolconfig` |
| `configs/{id}/{phase}.cfg` | Commandes RCON par phase | `matchconfig` |
| `configs/server_init.cfg` | CFG poussé à l'ajout serveur | `matchconfig` |
