<script>
import config from '@/config/matchmaking'
import { useAuthStore } from '@/stores/auth'

export default {
	data() {
		return {
			teams: [],
			players: [],
			profiles: {},
			loading: true,
			openTeam: null,
			showCreateModal: false,
			newTeamName: '',
			creating: false,
			createError: '',
			showAddPlayer: null, // teamId en cours d'ajout
		}
	},
	computed: {
		teamStats() {
			const stats = {}
			for (const team of this.teams) {
				const cs2Ratings = team.players
					.map(s => this.profiles[s])
					.filter(p => p?.cs2_status === 'ready' && p.premier_rating > 0)
					.map(p => p.premier_rating)
				const faceitElos = team.players
					.map(s => this.profiles[s])
					.filter(p => p?.faceit_status === 'ready' && p.faceit_elo > 0)
					.map(p => p.faceit_elo)
				stats[team.id] = {
					cs2Avg: cs2Ratings.length ? Math.round(cs2Ratings.reduce((a, b) => a + b, 0) / cs2Ratings.length) : null,
					faceitAvg: faceitElos.length ? Math.round(faceitElos.reduce((a, b) => a + b, 0) / faceitElos.length) : null,
				}
			}
			return stats
		},
		// Joueurs pas encore dans l'équipe ouverte
		availablePlayers() {
			if (!this.showAddPlayer) return []
			const team = this.teams.find(t => t.id === this.showAddPlayer)
			if (!team) return []
			return this.players.filter(p => !team.players.includes(p.steamid))
		},
	},
	mounted() {
		this.fetchAll()
	},
	methods: {
		authHeaders() {
			return { Authorization: `Bearer ${useAuthStore().token}` }
		},
		async fetchAll() {
			this.loading = true
			try {
				const [teamsRes, playersRes] = await Promise.all([
					fetch(`${config.api.baseUrl}/teams`, { headers: this.authHeaders() }),
					fetch(`${config.api.baseUrl}/players`, { headers: this.authHeaders() }),
				])
				this.teams = teamsRes.ok ? await teamsRes.json() : []
				this.players = playersRes.ok ? await playersRes.json() : []
				await this.fetchAllProfiles()
			} catch {
				this.teams = []
				this.players = []
			} finally {
				this.loading = false
			}
		},
		async fetchAllProfiles() {
			// Only fetch profiles for players that are in at least one team
			const inTeams = new Set(this.teams.flatMap(t => t.players))
			await Promise.all([...inTeams].map(async steamid => {
				try {
					const r = await fetch(`${config.api.baseUrl}/profile/${encodeURIComponent(steamid)}`, { headers: this.authHeaders() })
					if (r.ok) this.profiles = { ...this.profiles, [steamid]: await r.json() }
				} catch {}
			}))
		},
		playerInfo(steamid) {
			return this.players.find(p => p.steamid === steamid)
		},
		toggle(id) {
			this.openTeam = this.openTeam === id ? null : id
			this.showAddPlayer = null
		},
		openCreateModal() {
			this.newTeamName = ''
			this.createError = ''
			this.showCreateModal = true
		},
		async createTeam() {
			if (!this.newTeamName.trim()) return
			this.creating = true
			this.createError = ''
			try {
				const res = await fetch(`${config.api.baseUrl}/teams`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ name: this.newTeamName.trim() }),
				})
				if (res.ok) {
					this.showCreateModal = false
					await this.fetchAll()
				} else {
					const d = await res.json()
					this.createError = d.error || 'Erreur inconnue'
				}
			} finally {
				this.creating = false
			}
		},
		async deleteTeam(id) {
			await fetch(`${config.api.baseUrl}/teams/${id}`, {
				method: 'DELETE',
				headers: this.authHeaders(),
			})
			if (this.openTeam === id) this.openTeam = null
			await this.fetchAll()
		},
		async addPlayer(teamId, steamid) {
			await fetch(`${config.api.baseUrl}/teams/${teamId}/players`, {
				method: 'POST',
				headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
				body: JSON.stringify({ steamid }),
			})
			this.showAddPlayer = null
			await this.fetchAll()
		},
		async removePlayer(teamId, steamid) {
			await fetch(`${config.api.baseUrl}/teams/${teamId}/players/${encodeURIComponent(steamid)}`, {
				method: 'DELETE',
				headers: this.authHeaders(),
			})
			await this.fetchAll()
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item">Matchmaking</li>
		<li class="breadcrumb-item active">Équipes</li>
	</ul>

	<card>
		<card-header class="d-flex align-items-center justify-content-between fw-semibold">
			<div class="d-flex align-items-center gap-2">
				<i class="fa fa-shield-halved me-1 text-theme"></i>Équipes
				<span v-if="!loading" class="badge bg-inverse bg-opacity-15 text-inverse fw-normal">{{ teams.length }}</span>
			</div>
			<div class="d-flex align-items-center gap-2">
				<button class="btn btn-outline-theme btn-sm" @click="openCreateModal" title="Créer une équipe">
					<i class="fa fa-plus"></i>
				</button>
				<button class="btn btn-outline-theme btn-sm" @click="fetchAll" :disabled="loading" title="Actualiser">
					<i class="fa fa-rotate-right" :class="{ 'fa-spin': loading }"></i>
				</button>
			</div>
		</card-header>

		<div v-if="loading" class="py-5 text-center">
			<div class="spinner-border text-theme" role="status"></div>
		</div>

		<div v-else-if="teams.length === 0" class="py-5 d-flex flex-column align-items-center">
			<i class="fa fa-shield-halved fa-2x text-inverse text-opacity-15 mb-3"></i>
			<p class="text-inverse text-opacity-50 small mb-1">Aucune équipe créée.</p>
			<p class="text-inverse text-opacity-25 small mb-0">Utilisez <strong>+</strong> pour créer une équipe.</p>
		</div>

		<div v-else class="table-responsive">
			<table class="table table-hover mb-0">
				<thead>
					<tr>
						<th scope="col" style="width:16px"></th>
						<th scope="col">Équipe</th>
						<th scope="col">Niveau moyen</th>
						<th scope="col">Joueurs</th>
						<th scope="col"></th>
					</tr>
				</thead>
				<tbody>
					<template v-for="team in teams" :key="team.id">

						<!-- Ligne principale -->
						<tr
							class="align-middle"
							:class="{ 'table-active': openTeam === team.id }"
							style="cursor:pointer"
							@click="toggle(team.id)"
						>
							<td class="ps-3">
								<i class="fa fa-chevron-right fa-xs text-inverse text-opacity-25 transition-transform"
									:style="openTeam === team.id ? 'transform:rotate(90deg)' : ''"></i>
							</td>
							<td class="fw-semibold">{{ team.name }}</td>
							<td>
								<div class="d-flex align-items-center gap-2">
									<span v-if="teamStats[team.id]?.cs2Avg" class="badge bg-inverse bg-opacity-10 fw-normal" style="color:var(--bs-cyan);font-size:.72rem">
										<i class="fa fa-crosshairs me-1 opacity-50"></i>{{ teamStats[team.id].cs2Avg.toLocaleString() }}
									</span>
									<span v-if="teamStats[team.id]?.faceitAvg" class="badge bg-warning bg-opacity-15 text-warning fw-normal" style="font-size:.72rem">
										<i class="fa fa-fire me-1 opacity-50"></i>{{ teamStats[team.id].faceitAvg }} ELO
									</span>
								</div>
							</td>
							<td>
								<div class="d-flex align-items-center gap-1">
									<template v-for="steamid in team.players.slice(0, 8)" :key="steamid">
										<img
											v-if="playerInfo(steamid)?.avatar"
											:src="playerInfo(steamid).avatar"
											:title="playerInfo(steamid)?.username"
											width="22" height="22"
											style="border-radius:3px;flex-shrink:0"
										/>
										<div v-else
											:title="steamid"
											style="width:22px;height:22px;border-radius:3px;flex-shrink:0;background:rgba(255,255,255,.08)"
										></div>
									</template>
									<span v-if="team.players.length > 8" class="text-inverse text-opacity-50 small ms-1">
										+{{ team.players.length - 8 }}
									</span>
									<span v-if="team.players.length === 0" class="text-inverse text-opacity-25 small">
										Aucun joueur
									</span>
								</div>
							</td>
							<td class="text-end pe-3" @click.stop>
								<button class="btn btn-outline-danger btn-sm" @click="deleteTeam(team.id)" title="Supprimer">
									<i class="fa fa-trash"></i>
								</button>
							</td>
						</tr>

						<!-- Accordéon -->
						<tr v-if="openTeam === team.id">
							<td colspan="5" class="p-0">
								<div class="px-4 py-3 border-top border-light border-opacity-10">

									<div class="d-flex align-items-center justify-content-between mb-3">
										<p class="text-inverse text-opacity-50 text-uppercase fw-semibold mb-0" style="font-size:.65rem;letter-spacing:.1em">
											<i class="fa fa-users me-1"></i>Composition — {{ team.players.length }} joueur{{ team.players.length > 1 ? 's' : '' }}
										</p>
										<button
											v-if="availablePlayers.length > 0 || showAddPlayer !== team.id"
											class="btn btn-outline-theme btn-sm"
											@click="showAddPlayer = showAddPlayer === team.id ? null : team.id"
										>
											<i class="fa fa-plus me-1"></i>Ajouter
										</button>
									</div>

									<!-- Sélecteur d'ajout -->
									<transition name="fade">
										<div v-if="showAddPlayer === team.id" class="mb-3">
											<div v-if="availablePlayers.length === 0" class="text-inverse text-opacity-25 small">
												Tous les joueurs enregistrés sont déjà dans cette équipe.
											</div>
											<div v-else class="d-flex flex-wrap gap-2">
												<button
													v-for="p in availablePlayers" :key="p.steamid"
													class="btn btn-sm btn-outline-secondary d-flex align-items-center gap-2"
													@click="addPlayer(team.id, p.steamid)"
												>
													<img v-if="p.avatar" :src="p.avatar" width="18" height="18" style="border-radius:2px" />
													{{ p.username }}
												</button>
											</div>
										</div>
									</transition>

									<!-- Liste joueurs -->
									<div v-if="team.players.length === 0" class="text-inverse text-opacity-25 small">
										Aucun joueur dans cette équipe.
									</div>
									<table v-else class="table table-sm table-hover mb-0">
										<thead>
											<tr>
												<th scope="col">Joueur</th>
												<th scope="col">CS2 Premier</th>
												<th scope="col">Faceit</th>
												<th scope="col"></th>
											</tr>
										</thead>
										<tbody>
											<tr v-for="steamid in team.players" :key="steamid" class="align-middle">
												<td>
													<div class="d-flex align-items-center gap-2">
														<img
															v-if="playerInfo(steamid)?.avatar"
															:src="playerInfo(steamid).avatar"
															width="24" height="24"
															style="border-radius:3px;flex-shrink:0"
														/>
														<div v-else style="width:24px;height:24px;border-radius:3px;flex-shrink:0;background:rgba(255,255,255,.08)"></div>
														<div>
															<div class="fw-semibold lh-1">{{ playerInfo(steamid)?.username || steamid }}</div>
															<div class="font-monospace text-inverse text-opacity-25 mt-1" style="font-size:.7rem">{{ steamid }}</div>
														</div>
													</div>
												</td>
												<td>
													<template v-if="!profiles[steamid] || profiles[steamid].cs2_status === 'retrieving'">
														<span class="spinner-border spinner-border-sm text-inverse text-opacity-25" style="width:.75rem;height:.75rem;border-width:2px"></span>
													</template>
													<template v-else-if="profiles[steamid].cs2_status === 'ready'">
														<span v-if="profiles[steamid].premier_rating > 0" class="fw-semibold" style="color:var(--bs-cyan)">
															{{ profiles[steamid].premier_rating.toLocaleString() }}
														</span>
														<span v-else-if="profiles[steamid].competitive_rank > 0" class="text-inverse text-opacity-75 small">
															Rang {{ profiles[steamid].competitive_rank }}
														</span>
														<span v-else class="text-inverse text-opacity-25">—</span>
													</template>
													<span v-else class="text-inverse text-opacity-25">—</span>
												</td>
												<td>
													<template v-if="!profiles[steamid] || profiles[steamid].faceit_status === 'retrieving'">
														<span class="spinner-border spinner-border-sm text-inverse text-opacity-25" style="width:.75rem;height:.75rem;border-width:2px"></span>
													</template>
													<template v-else-if="profiles[steamid].faceit_status === 'ready'">
														<div class="d-flex align-items-center gap-2">
															<span class="badge bg-warning text-dark fw-bold" style="font-size:.72rem">
																LVL {{ profiles[steamid].faceit_level }}
															</span>
															<span class="text-inverse text-opacity-75 small">{{ profiles[steamid].faceit_elo }} ELO</span>
														</div>
													</template>
													<span v-else class="text-inverse text-opacity-25">—</span>
												</td>
												<td class="text-end">
													<button class="btn btn-outline-danger btn-sm" @click="removePlayer(team.id, steamid)" title="Retirer">
														<i class="fa fa-xmark"></i>
													</button>
												</td>
											</tr>
										</tbody>
									</table>

								</div>
							</td>
						</tr>

					</template>
				</tbody>
			</table>
		</div>
	</card>

	<!-- Modal création -->
	<teleport to="body">
		<transition name="fade">
			<div v-if="showCreateModal" class="modal-backdrop show" style="z-index:1040" @click="showCreateModal = false"></div>
		</transition>
		<transition name="slide-up">
			<div v-if="showCreateModal" class="modal d-block" style="z-index:1050" tabindex="-1" @click.self="showCreateModal = false">
				<div class="modal-dialog modal-dialog-centered">
					<div class="modal-content">
						<div class="modal-header border-0 pb-0">
							<h5 class="modal-title fw-semibold">
								<i class="fa fa-shield-halved me-2 text-theme"></i>Créer une équipe
							</h5>
							<button type="button" class="btn-close btn-close-white opacity-25" @click="showCreateModal = false"></button>
						</div>
						<div class="modal-body pt-3">
							<div class="mb-3">
								<label class="form-label small fw-semibold">Nom de l'équipe</label>
								<input v-model="newTeamName" type="text" class="form-control" placeholder="Team Alpha…" @keyup.enter="createTeam" autofocus />
							</div>
							<div v-if="createError" class="alert alert-danger py-2 small">
								<i class="fa fa-triangle-exclamation me-2"></i>{{ createError }}
							</div>
						</div>
						<div class="modal-footer border-0 pt-0">
							<button type="button" class="btn btn-outline-secondary" @click="showCreateModal = false">Annuler</button>
							<button type="button" class="btn btn-theme" @click="createTeam" :disabled="creating || !newTeamName.trim()">
								<span v-if="creating" class="spinner-border spinner-border-sm me-2"></span>
								<i v-else class="fa fa-plus me-2"></i>Créer
							</button>
						</div>
					</div>
				</div>
			</div>
		</transition>
	</teleport>
</template>

<style scoped>
.transition-transform { transition: transform .2s ease; }
.fade-enter-active, .fade-leave-active { transition: opacity .2s }
.fade-enter-from, .fade-leave-to { opacity: 0 }
.slide-up-enter-active, .slide-up-leave-active { transition: opacity .2s, transform .2s }
.slide-up-enter-from, .slide-up-leave-to { opacity: 0; transform: translateY(16px) }
</style>
