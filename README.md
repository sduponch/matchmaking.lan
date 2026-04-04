# matchmaking.lan

Plateforme de matchmaking CS2 pour LAN. Tableau de bord en temps réel avec suivi des matchs, gestion des serveurs, gestion des joueurs/équipes, authentification Steam et gestion de tournois.

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
> `ADMIN_STEAM_IDS` est reconcilié avec `players.json` à chaque redémarrage — pas besoin de reconnecter les joueurs pour que le rôle `admin` prenne effet.

## Fonctionnalités

### Authentification
- Login via Steam OpenID (popup)
- JWT avec rôle `admin` ou `player`
- Admins définis par Steam64 ID dans `.env`
- Enregistrement automatique du joueur à chaque connexion (`players.json`)
- Synchronisation des rôles au démarrage : modifier `ADMIN_STEAM_IDS` + redémarrer suffit

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
- **Token unique par serveur** (16 octets hex) — sert à la fois d'identifiant stable et à l'identification des logs dans l'URL
- Nom personnalisable depuis l'interface admin (édition inline, synchronisé via `hostname` RCON)
- Config init poussée automatiquement à l'ajout (mode Deathmatch, hostname "Warmup deathmatch", GOTV)
- Vérification de réception des logs à l'ajout (timeout 5s)
- `last_log_at` en mémoire — détection de serveurs plantés (vert <2min, orange <10min, rouge >10min)
- Changement de map à distance via RCON
- Comptage joueurs humains (hors bots)
- **Dropdown "Configurer"** — pousse `server_init.cfg` ou le warmup d'un profil de match via RCON

### Profils de match
- CRUD complet via l'interface admin (Serveurs → Profils de match)
- CFG éditable par phase : `warmup`, `knife`, `live`, `halftime`, `game_over`
- Éditeur textarea monospace avec sauvegarde par onglet
- `server_init.cfg` éditable depuis l'interface (config poussée à l'ajout de chaque serveur)
- Commentaires `//` supportés dans les CFGs (ignorés à l'envoi RCON)
- Profil par défaut : "5v5 Compétitif" avec CFGs warmup/knife/live préconfigurés
- **Tags de mode de jeu** — associe un profil à un ou plusieurs modes (defuse, wingman, etc.) ; vide = compatible tous modes

### Rencontres (Encounters)
- Création de rencontres standalone Team A vs Team B (BO1 / BO3 / BO5)
- **7 modes de jeu** : Défuse, Occasionnel, Wingman, Reprise de contrôle, Otages, Arms Race, Deathmatch
- **Choix de la carte** :
  - Manuel — sélection directe depuis le pool des serveurs (filtré par mode) ou saisie libre
  - Pick & Ban — pool de cartes éligibles configurable (maps officielles pré-remplies par mode)
- **Veto (si Pick & Ban)** : qui commence (Seed / Aléatoire / Challenge) ; camp sur la carte décisive (Pick & Ban / Aléatoire / Knife / Vote joueurs)
- **Choix du côté de départ** (si manuel) : Round couteaux / CT fixe / T fixe
- **Mode de lancement** : Manuel (admin) / Planifié (date + heure) / Ready (`!ready` en jeu)
- Démarrage : assignation serveur + profil, push warmup CFG + game mode, hostname avec noms d'équipes + labels CT/T, changelevel
- **Phases automatiques** : warmup → couteaux (si `!ready` + knife) → mi-temps → 2ème mi-temps → game over ; chaque phase déclenche le CFG et le hostname correspondants via RCON
- **Hostname dynamique** par phase : `"TeamA (CT) vs TeamB (T) - 1ère Mi-temps"`, `"... - Couteaux"`, etc.
- **Correction automatique du côté** : au premier joueur humain connecté, si le côté est incorrect (ex: CT attendu mais T reçu), `mp_swapteams` est envoyé puis les noms d'équipes sont re-poussés 2s après
- Re-push warmup CFG après chaque changelevel (CS2 reset les ConVars à chaque map)
- Démo automatique : `tv_record` déclenché à la fin du premier warmup
- Override résultat admin par map
- Réinitialisation d'une rencontre live ou terminée

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
- Joueurs (connect, disconnect, switch équipe — détection premier joueur humain)
- Chat (global, équipe) — commandes `!ready`, `!ct`, `!t`
- Score, statut match, warmup start/end, freeze period
- Map loading (`Loading map "..."`) et map started (`Started:  "..."` avec chemin +prefabs)

## Architecture technique

### Hiérarchie tournoi (en cours d'implémentation)

```
Tournament (Tournoi)
  └── Phase  (Poules / Suisse / Upper bracket / Lower bracket)
        └── Round  (Ronde 1, Ronde 2…)
              └── Encounter  (Rencontre — Team A vs Team B, bo1/bo3/bo5)
                    └── GameMap  (Map 1, Map 2, Map 3)
```

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
    ├─→ gamelog.Broker.publish() → SSE /servers/:token/logs
    └─→ gamelog.OnEvent() → match.Apply()
                                │
                                ▼
                        match.Machine (par serveur)
                        MatchState {phase, map, round, score, players}
```

### Hooks (câblage dans main.go)

| Hook | Rôle |
|------|------|
| `gamelog.OnEvent` | → `match.Apply` — dispatch événements vers la machine d'état |
| `gamelog.OnLog` | → `server.UpdateLastLog` — mise à jour `last_log_at` |
| `gamelog.ResolveToken` | → `server.GetAddrByToken` — résolution token → addr |
| `match.GetLastLogAt` | → `server.GetLastLogAt` — inclus dans `GET /servers/:token/match` |
| `match.GetEncounterInfo` | → `encounter.GetByServerID` — sidePick / readyCount / maxRounds |
| `match.OnGameOver` | → `encounter.RecordResult` — résultat map + calcul gagnant bo1/3/5 |
| `match.OnPhaseChange` | push CFG + hostname par phase (warmup/knife/first_half/halftime/second_half/overtime/game_over) |
| `match.OnKnifeChoice` | `mp_swapteams` si nécessaire + push live.cfg + hostname |
| `match.OnFirstPlayerJoin` | swap côté si mauvais équipe + push teamnames 2s après |
| `encounter.OnStart` | → `match.ExpectWarmup` — arme le flag avant changelevel |
| `encounter.OnComplete` | → `phase.CheckRoundComplete` *(à venir)* |
| `phase.OnComplete` | → `tournament.Advance` *(à venir)* |

### Persistance

| Fichier | Contenu |
|---------|---------|
| `servers.json` | `map[token]{addr, name, rcon, token}` — token = identifiant stable |
| `players.json` | `map[steamid]{username, avatar, role, team, last_seen}` |
| `teams.json` | `map[id]{name, players[]}` |
| `match_profiles.json` | `map[id]{name, tags[], created_at}` — tags = modes de jeu compatibles (vide = tous) |
| `configs/server_init.cfg` | config poussée à l'ajout de chaque serveur (deathmatch par défaut) |
| `configs/{profile_id}/{phase}.cfg` | CFG RCON par profil et par phase |
| `encounters.json` | `map[id]{team1, team2, format, game_mode, side_pick, launch_mode, pick_ban, map_pool, veto_first, decider_side, status, maps[], ...}` |
| `phases.json` | phases de tournoi (à venir) |
| `tournaments.json` | tournois (à venir) |
| `matches/{encounter_id}.json` | historique round par round (à venir) |

`last_log_at` est **en mémoire uniquement** (pas de persistance disque).

**Migrations automatiques `servers.json` :**
- v1 `map[addr]string` (rcon) → v2 `map[addr]{rcon, token}` → v3 `map[token]{addr, name, rcon, token}`

### Identification serveur derrière NAT

À l'ajout d'un serveur, le backend envoie via RCON :
```
logaddress_add_http "https://api.example.com/internal/log/<token>"
```
Le token (16 octets hex aléatoires) est persisté dans `servers.json`. Il est extrait du path à chaque POST et résolu en adresse serveur — fonctionne même si tous les serveurs LAN partagent la même IP publique.

Le token sert aussi d'**identifiant stable** des serveurs dans toutes les routes API (`/servers/:token/*`) et dans les rencontres/tournois, indépendamment des changements d'adresse IP.

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

### Auth & Profils

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/steam` | — | Callback OpenID Steam |
| GET | `/auth/me` | ✓ | Infos joueur connecté |
| GET | `/profile/:steamid` | ✓ | Profil Steam complet |
| GET | `/profile/:steamid/cs2` | ✓ | Stats CS2 |
| GET | `/profile/:steamid/faceit` | ✓ | Stats Faceit |

### Joueurs & Équipes

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/players` | admin | Liste joueurs enregistrés |
| GET | `/teams` | ✓ | Liste équipes |
| POST | `/teams` | admin | Créer une équipe |
| DELETE | `/teams/:id` | admin | Supprimer une équipe |
| POST | `/teams/:id/players` | admin | Ajouter joueur à l'équipe |
| DELETE | `/teams/:id/players/:steamid` | admin | Retirer joueur de l'équipe |

### Serveurs

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/servers` | ✓ | Liste serveurs LAN |
| POST | `/servers` | admin | Ajouter serveur (RCON + vérif logs + server_init.cfg) |
| DELETE | `/servers/:token` | admin | Retirer serveur |
| PUT | `/servers/:token/name` | admin | Renommer le serveur (+ hostname RCON) |
| POST | `/servers/:token/map` | admin | Changer la map |
| POST | `/servers/:token/cfg` | admin | Pousser server_init ou warmup d'un profil |
| GET | `/servers/:token/match` | ✓ | État du match + last_log_at |
| GET | `/servers/:token/logs` | ✓ | SSE événements logs |
| POST | `/internal/log/:token` | — | Réception logs CS2 |

### Profils de match

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/match-profiles` | ✓ | Liste profils (avec CFGs) |
| POST | `/match-profiles` | admin | Créer un profil |
| GET | `/match-profiles/:id` | ✓ | Détail profil |
| PUT | `/match-profiles/:id` | admin | Modifier métadonnées |
| DELETE | `/match-profiles/:id` | admin | Supprimer profil + CFGs |
| GET | `/match-profiles/:id/cfg/:phase` | ✓ | Contenu CFG d'une phase |
| PUT | `/match-profiles/:id/cfg/:phase` | admin | Écrire CFG d'une phase |
| GET | `/server-init-cfg` | ✓ | Contenu server_init.cfg |
| PUT | `/server-init-cfg` | admin | Écrire server_init.cfg |

### Tournois *(à venir)*

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/encounters` | ✓ | Liste rencontres |
| POST | `/encounters` | admin | Créer une rencontre |
| GET | `/encounters/:id` | ✓ | Détail rencontre |
| PUT | `/encounters/:id` | admin | Modifier métadonnées |
| POST | `/encounters/:id/start` | admin | Assigner serveur + profil, démarrer |
| POST | `/encounters/:id/reopen` | admin | Réinitialiser (live/completed → scheduled) |
| POST | `/encounters/:id/result` | admin | Override résultat d'une map |
| DELETE | `/encounters/:id` | admin | Supprimer |
| GET | `/encounters/:id/rounds` | ✓ | Historique round par round |
| GET/POST | `/phases` | admin | Phases de tournoi |
| POST | `/phases/:id/seed` | admin | Seeder les équipes |
| POST | `/phases/:id/rounds/generate` | admin | Générer la prochaine ronde |
| GET/POST | `/tournaments` | admin | Tournois |
| POST | `/tournaments/:id/start` | admin | Démarrer le tournoi |

## Plan d'implémentation

```
✅ Auth Steam + JWT
✅ Registre joueurs (players.json) + sync rôles au démarrage
✅ Gestion équipes (teams.json)
✅ Stats CS2 + Faceit (cache)
✅ Serveurs LAN (A2S + RCON + logaddress token)
✅ Machine d'état match (gamelog → phases → stats)
✅ Refacto server : token comme ID stable + Name éditable
✅ Match Config Profiles + CFG editor (server_init.cfg deathmatch + phases warmup/knife/live/halftime/game_over)

🔄 Encounter : rencontre standalone + intégration CS2 + démos
   ✅ CRUD + démarrage (RCON warmup/changelevel/hostname/tv_record)
   ✅ Modes de jeu (defuse/casual/wingman/retakes/hostage/armsrace/deathmatch)
   ✅ Pick & Ban config (pool officiel, veto_first, decider_side)
   ✅ Choix côté de départ (knife/ct/t ou règles pick&ban)
   ✅ Mode de lancement (manuel/planifié/ready)
   ✅ RecordResult + ReOpen + SetResult admin
   ✅ Hostname dynamique avec noms d'équipes + labels CT/T par phase
   ✅ Correction automatique du côté au premier joueur (mp_swapteams)
   ✅ Déclenchement automatique des phases (knife, first_half, halftime, second_half)
   ⬜ Logique pick & ban interactive (interface joueurs)
⬜ Round history : stats round par round persistées
⬜ Phase : poules avec seeding Faceit/CS2
⬜ Phase : ronde suisse
⬜ Phase : brackets upper/lower (double élimination)
⬜ Tournament : orchestration + avancement automatique
⬜ Frontend : vues par couche (encounters → phases → tournoi)
```
