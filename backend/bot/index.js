const http = require('http');
const SteamUser = require('steam-user');
const GlobalOffensive = require('globaloffensive');

const client = new SteamUser();
const cs2 = new GlobalOffensive(client);

const PORT            = process.env.BOT_PORT || 3001;
const CACHE_TTL       = 5 * 60 * 1000; // 5 min
const GAME_SERVER     = process.env.GAME_SERVER_ADDR || '';

const cache = new Map();   // steamid -> { data, ts }
let gcReady = false;

function log(...args) {
  const ts = new Date().toISOString().replace('T', ' ').slice(0, 19).replace(/-/g, '/');
  process.stdout.write(ts + ' ' + args.join(' ') + '\n');
}

function logError(...args) {
  const ts = new Date().toISOString().replace('T', ' ').slice(0, 19).replace(/-/g, '/');
  process.stderr.write(ts + ' ' + args.join(' ') + '\n');
}

// --- Steam login ---

const logOnOptions = {
  accountName: process.env.BOT_USERNAME,
  password:    process.env.BOT_PASSWORD,
};
if (process.env.BOT_SHARED_SECRET) {
  const steamTotp = require('steam-totp');
  logOnOptions.twoFactorCode = steamTotp.generateAuthCode(process.env.BOT_SHARED_SECRET);
}

client.logOn(logOnOptions);

client.on('loggedOn', () => {
  log('[bot] Logged into Steam');
  client.setPersona(SteamUser.EPersonaState.Invisible);
  client.gamesPlayed([730]);
});

client.on('error', (err) => {
  const throttled = err.message.includes('RateLimitExceeded') || err.message.includes('DeniedThrottle');
  if (throttled) {
    logError('[bot] Steam throttle — nouvel essai dans 30 minutes');
    setTimeout(() => process.exit(1), 30 * 60 * 1000);
  } else {
    logError('[bot] Steam error:', err.message);
    process.exit(1);
  }
});

// Accepte automatiquement les demandes d'amis
client.on('friendRelationship', (steamid, relationship) => {
  if (relationship === SteamUser.EFriendRelationship.RequestRecipient) {
    client.addFriend(steamid);
    log('[bot] Accepted friend request from', steamid.toString());
  }
});

cs2.on('connectedToGC', () => {
  log('[bot] Connected to CS2 GC');
  gcReady = true;
});

cs2.on('disconnectedFromGC', (reason) => {
  logError('[bot] Disconnected from GC:', reason);
  gcReady = false;
});

// --- Rank fetching ---

// rank_type_id : 6 = Compétitif | 7 = Wingman | 11 = Premier
function extractRank(profile, rankTypeId) {
  if (Array.isArray(profile?.rankings)) {
    const entry = profile.rankings.find(r => r.rank_type_id === rankTypeId);
    if (entry) return { rank_id: entry.rank_id ?? 0, wins: entry.wins ?? 0 };
  }
  return { rank_id: 0, wins: 0 };
}

function getRank(steamid) {
  return new Promise((resolve, reject) => {
    const cached = cache.get(steamid);
    if (cached && Date.now() - cached.ts < CACHE_TTL) {
      return resolve(cached.data);
    }

    if (!gcReady) {
      return reject(new Error('GC not ready'));
    }

    const timer = setTimeout(() => {
      const relationship = client.myFriends[steamid];
      if (relationship !== SteamUser.EFriendRelationship.Friend) {
        if (relationship !== SteamUser.EFriendRelationship.RequestInitiator) {
          client.addFriend(steamid);
          log('[bot] Friend request sent to', steamid);
        } else {
          log('[bot] Friend request already pending for', steamid);
        }
      }
      reject(new Error('GC request timeout'));
    }, 10_000);

    cs2.requestPlayersProfile(steamid, (profile) => {
      clearTimeout(timer);

      const premier     = extractRank(profile, 11);
      const competitive = extractRank(profile, 6);

      const data = {
        premier_rating:   premier.rank_id,
        competitive_rank: competitive.rank_id,
        competitive_wins: competitive.wins,
      };

      log('[bot] Rank fetched for', steamid, JSON.stringify(data));
      cache.set(steamid, { data, ts: Date.now() });

      if (GAME_SERVER) {
        const lines = ['✅ Ton profil CS2 a été synchronisé sur matchmaking.lan'];
        if (data.premier_rating > 0) lines.push(`🏆 Premier : ${data.premier_rating.toLocaleString()} pts`);
        const connectUrl = (process.env.FRONTEND_URL || 'http://localhost:5173') + `/connect/${GAME_SERVER}`;
        lines.push(`\n🎮 Rejoindre le serveur : ${connectUrl}`);
        client.chat.sendFriendMessage(steamid, lines.join('\n'))
          .catch(err => logError('[bot] Failed to send message to', steamid, err.message));
      }

      resolve(data);
    });
  });
}

// --- HTTP server ---

const server = http.createServer(async (req, res) => {
  if (req.url === '/info') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ steamid: client.steamID?.getSteamID64() ?? null }));
    return;
  }

  const match = req.url.match(/^\/rank\/(\d+)$/);
  if (!match) {
    res.writeHead(404).end(JSON.stringify({ error: 'not found' }));
    return;
  }

  try {
    const data = await getRank(match[1]);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(data));
  } catch (e) {
    res.writeHead(503, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: e.message }));
  }
});

server.listen(PORT, '127.0.0.1', () => {
  log(`[bot] Listening on :${PORT}`);
});
