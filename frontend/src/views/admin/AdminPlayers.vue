<script>
import config from '@/config/matchmaking'
import { useAuthStore } from '@/stores/auth'

export default {
	data() {
		return {
			players: [],
			profiles: {},
			loading: true,
			_pollTimer: null,
		}
	},
	mounted() {
		this.fetchPlayers()
	},
	beforeUnmount() {
		clearInterval(this._pollTimer)
	},
	methods: {
		authHeaders() {
			return { Authorization: `Bearer ${useAuthStore().token}` }
		},
		async fetchPlayers() {
			this.loading = true
			clearInterval(this._pollTimer)
			try {
				const res = await fetch(`${config.api.baseUrl}/players`, { headers: this.authHeaders() })
				if (res.ok) {
					this.players = await res.json()
					await this.fetchAllProfiles()
					this.startPoll()
				} else {
					this.players = []
				}
			} catch {
				this.players = []
			} finally {
				this.loading = false
			}
		},
		async fetchAllProfiles() {
			await Promise.all(this.players.map(p => this.fetchProfile(p.steamid)))
		},
		async fetchProfile(steamid) {
			try {
				const res = await fetch(`${config.api.baseUrl}/profile/${encodeURIComponent(steamid)}`, { headers: this.authHeaders() })
				if (res.ok) {
					this.profiles = { ...this.profiles, [steamid]: await res.json() }
				}
			} catch {}
		},
		async pollPending() {
			const pending = this.players.filter(p => {
				const pr = this.profiles[p.steamid]
				return !pr || pr.cs2_status === 'retrieving' || pr.faceit_status === 'retrieving'
			})
			if (pending.length === 0) {
				clearInterval(this._pollTimer)
				return
			}
			await Promise.all(pending.map(async p => {
				const pr = this.profiles[p.steamid]
				if (!pr) return this.fetchProfile(p.steamid)
				if (pr.cs2_status === 'retrieving') {
					try {
						const r = await fetch(`${config.api.baseUrl}/profile/${encodeURIComponent(p.steamid)}/cs2`, { headers: this.authHeaders() })
						if (r.ok) {
							const d = await r.json()
							this.profiles = { ...this.profiles, [p.steamid]: { ...pr, cs2_status: d.status, premier_rating: d.premier_rating, competitive_rank: d.competitive_rank, competitive_wins: d.competitive_wins } }
						}
					} catch {}
				}
				if (pr.faceit_status === 'retrieving') {
					try {
						const r = await fetch(`${config.api.baseUrl}/profile/${encodeURIComponent(p.steamid)}/faceit`, { headers: this.authHeaders() })
						if (r.ok) {
							const d = await r.json()
							this.profiles = { ...this.profiles, [p.steamid]: { ...this.profiles[p.steamid], faceit_status: d.status, faceit_elo: d.faceit_elo, faceit_level: d.faceit_level, faceit_nickname: d.faceit_nickname } }
						}
					} catch {}
				}
			}))
		},
		startPoll() {
			this._pollTimer = setInterval(() => this.pollPending(), 2500)
		},
		formatDate(iso) {
			return new Date(iso).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })
		},
		cs2Label(pr) {
			if (!pr) return null
			if (pr.cs2_status === 'retrieving') return 'retrieving'
			if (pr.cs2_status !== 'ready') return null
			if (pr.premier_rating > 0) return pr.premier_rating.toLocaleString()
			if (pr.competitive_rank > 0) return `Rang ${pr.competitive_rank}`
			return '—'
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item active">Joueurs</li>
	</ul>

	<card>
		<card-header class="d-flex align-items-center justify-content-between fw-semibold">
			<div class="d-flex align-items-center gap-2">
				<i class="fa fa-users me-1 text-theme"></i>Joueurs inscrits
				<span v-if="!loading" class="badge bg-inverse bg-opacity-15 text-inverse fw-normal">{{ players.length }}</span>
			</div>
			<button class="btn btn-outline-theme btn-sm" @click="fetchPlayers" :disabled="loading" title="Actualiser">
				<i class="fa fa-rotate-right" :class="{ 'fa-spin': loading }"></i>
			</button>
		</card-header>

		<div v-if="loading" class="py-5 text-center">
			<div class="spinner-border text-theme" role="status"></div>
		</div>

		<div v-else-if="players.length === 0" class="py-5 d-flex flex-column align-items-center">
			<i class="fa fa-users fa-2x text-inverse text-opacity-15 mb-3"></i>
			<p class="text-inverse text-opacity-50 small mb-0">Aucun joueur enregistré.</p>
		</div>

		<div v-else class="table-responsive">
			<table class="table table-hover mb-0">
				<thead>
					<tr>
						<th scope="col">Joueur</th>
						<th scope="col">Rôle</th>
						<th scope="col">Équipe</th>
						<th scope="col">CS2 Premier</th>
						<th scope="col">Faceit</th>
						<th scope="col">Dernière connexion</th>
					</tr>
				</thead>
				<tbody>
					<tr v-for="p in players" :key="p.steamid" class="align-middle">
						<td>
							<div class="d-flex align-items-center gap-2">
								<img v-if="p.avatar" :src="p.avatar" width="28" height="28" style="border-radius:4px;flex-shrink:0" />
								<div v-else style="width:28px;height:28px;border-radius:4px;flex-shrink:0;background:rgba(255,255,255,.08)"></div>
								<div>
									<div class="fw-semibold lh-1">{{ p.username }}</div>
									<div class="font-monospace text-inverse text-opacity-25 mt-1" style="font-size:.72rem">{{ p.steamid }}</div>
								</div>
							</div>
						</td>
						<td>
							<span class="badge" :class="p.role === 'admin' ? 'bg-theme text-theme-900' : 'bg-inverse bg-opacity-15 text-inverse'">
								{{ p.role }}
							</span>
						</td>
						<td>
							<span v-if="p.team" class="badge bg-inverse bg-opacity-15 text-inverse">{{ p.team }}</span>
							<span v-else class="text-inverse text-opacity-25">—</span>
						</td>
						<td>
							<template v-if="!profiles[p.steamid] || profiles[p.steamid].cs2_status === 'retrieving'">
								<span class="spinner-border spinner-border-sm text-inverse text-opacity-25" style="width:.75rem;height:.75rem;border-width:2px"></span>
							</template>
							<template v-else-if="profiles[p.steamid].cs2_status === 'ready'">
								<span v-if="profiles[p.steamid].premier_rating > 0" class="fw-semibold" style="color:var(--bs-cyan)">
									{{ profiles[p.steamid].premier_rating.toLocaleString() }}
								</span>
								<span v-else-if="profiles[p.steamid].competitive_rank > 0" class="text-inverse text-opacity-75 small">
									Rang {{ profiles[p.steamid].competitive_rank }}
								</span>
								<span v-else class="text-inverse text-opacity-25">—</span>
							</template>
							<span v-else class="text-inverse text-opacity-25">—</span>
						</td>
						<td>
							<template v-if="!profiles[p.steamid] || profiles[p.steamid].faceit_status === 'retrieving'">
								<span class="spinner-border spinner-border-sm text-inverse text-opacity-25" style="width:.75rem;height:.75rem;border-width:2px"></span>
							</template>
							<template v-else-if="profiles[p.steamid].faceit_status === 'ready'">
								<div class="d-flex align-items-center gap-2">
									<span class="badge bg-warning text-dark fw-bold" style="font-size:.72rem">
										LVL {{ profiles[p.steamid].faceit_level }}
									</span>
									<span class="text-inverse text-opacity-75 small">{{ profiles[p.steamid].faceit_elo }} ELO</span>
								</div>
							</template>
							<span v-else class="text-inverse text-opacity-25">—</span>
						</td>
						<td class="text-inverse text-opacity-50 small">{{ formatDate(p.last_seen) }}</td>
					</tr>
				</tbody>
			</table>
		</div>
	</card>
</template>
