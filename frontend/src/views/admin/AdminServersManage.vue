<script>
import config from '@/config/matchmaking'
import { useAuthStore } from '@/stores/auth'

export default {
	data() {
		return {
			servers: [],
			matchStates: {},
			avatarCache: {},
			loading: true,
			mapSelections: {},
			changingMap: {},
			matchPollInterval: null,
			openServer: null,
			editingName: null,  // token of server being renamed
			editNameValue: '',
			profiles: [],
			pushingCfg: {},     // serverId → bool
		}
	},
	setup() {
		const cs2Maps = [
			'de_ancient', 'de_anubis', 'de_dust2', 'de_inferno',
			'de_mirage', 'de_nuke', 'de_overpass', 'de_train', 'de_vertigo',
		]
		return { cs2Maps }
	},
	computed: {
		managedServers() {
			return (this.servers || []).filter(s => s.managed)
		},
	},
	mounted() {
		this.fetchServers()
		this.fetchProfiles()
		this.matchPollInterval = setInterval(() => this.fetchMatchStates(), 5000)
	},
	unmounted() {
		clearInterval(this.matchPollInterval)
	},
	methods: {
		authHeaders() {
			return { Authorization: `Bearer ${useAuthStore().token}` }
		},
		async fetchServers() {
			this.loading = true
			try {
				const res = await fetch(`${config.api.baseUrl}/servers`, { headers: this.authHeaders() })
				this.servers = await res.json()
				await this.fetchMatchStates()
			} catch {
				this.servers = []
			} finally {
				this.loading = false
			}
		},
		async fetchMatchStates() {
			const managed = (this.servers || []).filter(s => s.managed && s.online)
			const results = await Promise.all(
				managed.map(s =>
					fetch(`${config.api.baseUrl}/servers/${s.id}/match`, { headers: this.authHeaders() })
						.then(r => r.json())
						.then(state => ({ id: s.id, addr: s.addr, state }))
						.catch(() => null)
				)
			)
			const map = {}
			for (const r of results) {
				if (r) {
					map[r.id] = r.state
					this.prefetchAvatars(r.state)
					const srv = this.servers.find(s => s.id === r.id)
					if (srv && r.state.last_log_at) srv.last_log_at = r.state.last_log_at
				}
			}
			this.matchStates = map
		},
		prefetchAvatars(state) {
			for (const p of Object.values(state.players ?? {})) {
				if (p.steamid && /^\d{17}$/.test(p.steamid) && !this.avatarCache[p.steamid]) {
					this.avatarCache[p.steamid] = ''
					fetch(`${config.api.baseUrl}/profile/${encodeURIComponent(p.steamid)}`, { headers: this.authHeaders() })
						.then(r => r.json())
						.then(profile => { this.avatarCache[p.steamid] = profile.avatar || '' })
						.catch(() => {})
				}
			}
		},
		toggle(id) {
			this.openServer = this.openServer === id ? null : id
		},
		displayName(srv) {
			return srv.name || srv.addr
		},
		startEditName(srv) {
			this.editingName = srv.id
			this.editNameValue = srv.name || ''
		},
		cancelEditName() {
			this.editingName = null
			this.editNameValue = ''
		},
		async saveEditName(srv) {
			const name = this.editNameValue.trim()
			if (!name) { this.cancelEditName(); return }
			await fetch(`${config.api.baseUrl}/servers/${srv.id}/name`, {
				method: 'PUT',
				headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
				body: JSON.stringify({ name }),
			})
			srv.name = name
			this.cancelEditName()
		},
		matchSteps(matchState) {
			const phase = matchState.phase
			const round = matchState.round
			const map = matchState.map
			const stepIndex = {
				idle:      0,
				warmup:    1,
				knife:     2,
				live:      2,
				halftime:  2,
				overtime:  2,
				game_over: 3,
			}[phase] ?? 0
			const steps = [
				{ key: 'idle',      label: 'En attente', detail: '—' },
				{ key: 'warmup',    label: 'Warmup',     detail: map || '—' },
				{ key: 'live',      label: 'En cours',   detail: round > 0 ? `Round ${round} / 24` : (map || '—') },
				{ key: 'game_over', label: 'Terminé',    detail: map || '—' },
			]
			return steps.map((s, i) => ({
				...s,
				state: i < stepIndex ? 'completed' : i === stepIndex ? 'active' : 'disabled',
			}))
		},
		phaseLabel(phase) {
			return { idle: '—', warmup: 'Warmup', knife: 'Couteaux', live: 'En cours', halftime: 'Mi-temps', overtime: 'Prolongation', game_over: 'Terminé' }[phase] ?? phase
		},
		phaseClass(phase) {
			return { live: 'text-theme', game_over: 'text-danger', warmup: 'text-warning', knife: 'text-warning' }[phase] ?? 'text-inverse text-opacity-50'
		},
		playersForTeam(matchState, team) {
			return Object.values(matchState.players ?? {})
				.filter(p => p.team === team)
				.sort((a, b) => b.kills - a.kills)
		},
		async changeMap(id) {
			const map = this.mapSelections[id]
			if (!map) return
			this.changingMap[id] = true
			try {
				await fetch(`${config.api.baseUrl}/servers/${id}/map`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ map }),
				})
				setTimeout(() => this.fetchServers(), 3000)
			} finally {
				this.changingMap[id] = false
			}
		},
		lastLogLabel(iso) {
			const diff = (Date.now() - new Date(iso).getTime()) / 1000
			if (diff < 60) return `il y a ${Math.floor(diff)}s`
			if (diff < 3600) return `il y a ${Math.floor(diff / 60)}min`
			if (diff < 86400) return `il y a ${Math.floor(diff / 3600)}h`
			return `il y a ${Math.floor(diff / 86400)}j`
		},
		lastLogClass(iso) {
			const diff = (Date.now() - new Date(iso).getTime()) / 1000
			if (diff < 120) return 'text-success'
			if (diff < 600) return 'text-warning'
			return 'text-danger'
		},
		async removeServer(srv) {
			await fetch(`${config.api.baseUrl}/servers/${srv.id}`, {
				method: 'DELETE',
				headers: this.authHeaders(),
			})
			if (this.openServer === srv.id) this.openServer = null
			await this.fetchServers()
		},
		async fetchProfiles() {
			try {
				const res = await fetch(`${config.api.baseUrl}/match-profiles`, { headers: this.authHeaders() })
				this.profiles = await res.json()
			} catch {
				this.profiles = []
			}
		},
		async pushCfg(srv, profileId) {
			this.pushingCfg[srv.id] = true
			try {
				await fetch(`${config.api.baseUrl}/servers/${srv.id}/cfg`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ profile_id: profileId }),
				})
			} finally {
				this.pushingCfg[srv.id] = false
			}
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item">Serveurs</li>
		<li class="breadcrumb-item active">Counter-Strike 2</li>
	</ul>
	<div v-if="loading" class="text-center py-5">
		<div class="spinner-border text-theme" role="status"></div>
	</div>

	<template v-else>
		<div v-if="managedServers.length === 0" class="alert alert-warning">
			Aucun serveur configuré. <router-link to="/admin/server/setup" class="alert-link">Ajoutez-en un depuis la page Configurer.</router-link>
		</div>

		<card v-else>
			<card-header class="d-flex align-items-center justify-content-between fw-semibold">
				<div class="d-flex align-items-center gap-2">
					<i class="fa fa-display me-1 text-theme"></i>Serveurs configurés
					<span class="badge bg-inverse bg-opacity-15 text-inverse fw-normal">{{ managedServers.length }}</span>
				</div>
				<button class="btn btn-outline-theme btn-sm" @click="fetchServers" :disabled="loading">
					<i class="fa fa-rotate-right" :class="{ 'fa-spin': loading }"></i>
				</button>
			</card-header>
			<div class="table-responsive">
				<table class="table table-hover mb-0">
					<thead>
						<tr>
							<th scope="col" style="width:16px"></th>
							<th scope="col">Nom</th>
							<th scope="col">Adresse</th>
							<th scope="col">Statut</th>
							<th scope="col">Score</th>
							<th scope="col">Map</th>
							<th scope="col">Joueurs</th>
							<th scope="col">Ping</th>
							<th scope="col">Dernier log</th>
							<th scope="col"></th>
						</tr>
					</thead>
					<tbody>
						<template v-for="srv in managedServers" :key="srv.id">

							<!-- Ligne principale -->
							<tr
								class="align-middle"
								:class="{ 'table-active': openServer === srv.id }"
								style="cursor:pointer"
								@click="toggle(srv.id)"
							>
								<td class="ps-3">
									<i class="fa fa-chevron-right fa-xs text-inverse text-opacity-25 transition-transform"
										:style="openServer === srv.id ? 'transform:rotate(90deg)' : ''">
									</i>
								</td>
								<td>
									<!-- Inline name editor -->
									<div v-if="editingName === srv.id" class="d-flex align-items-center gap-1" @click.stop>
										<input
											v-model="editNameValue"
											class="form-control form-control-sm"
											style="max-width:220px"
											@keyup.enter="saveEditName(srv)"
											@keyup.escape="cancelEditName"
											autofocus
										/>
										<button class="btn btn-sm btn-outline-theme px-2" @click="saveEditName(srv)"><i class="fa fa-check"></i></button>
										<button class="btn btn-sm btn-outline-secondary px-2" @click="cancelEditName"><i class="fa fa-xmark"></i></button>
									</div>
									<div v-else class="d-flex align-items-center gap-2">
										<span class="d-inline-block rounded-circle flex-shrink-0"
											:class="srv.online ? 'bg-success' : 'bg-danger'"
											style="width:7px;height:7px">
										</span>
										<span class="fw-semibold">{{ displayName(srv) }}</span>
										<button class="btn btn-link btn-sm p-0 text-inverse text-opacity-25" @click.stop="startEditName(srv)" title="Renommer">
											<i class="fa fa-pen fa-xs"></i>
										</button>
									</div>
								</td>
								<td class="font-monospace text-inverse text-opacity-50" style="font-size:.82rem">{{ srv.addr }}</td>
								<td>
									<template v-if="matchStates[srv.id] && matchStates[srv.id].phase !== 'idle'">
										<span class="small fw-semibold" :class="phaseClass(matchStates[srv.id].phase)">
											{{ phaseLabel(matchStates[srv.id].phase) }}
										</span>
										<span v-if="matchStates[srv.id].round > 0" class="text-inverse text-opacity-50 small ms-1">
											— Round {{ matchStates[srv.id].round }} / 24
										</span>
									</template>
									<template v-else>
										<span class="badge d-inline-flex align-items-center px-2 pt-5px pb-5px rounded fs-12px"
											:class="srv.online ? 'border border-success text-success' : 'border border-danger text-danger'">
											<i class="fa fa-circle fs-9px fa-fw me-5px"></i>{{ srv.online ? 'En ligne' : 'Hors ligne' }}
										</span>
									</template>
								</td>
								<td>
									<template v-if="matchStates[srv.id] && matchStates[srv.id].phase !== 'idle'">
										<span class="fw-bold" style="color:var(--bs-cyan)">{{ matchStates[srv.id].score_ct }}</span>
										<span class="text-inverse text-opacity-25 mx-1">:</span>
										<span class="fw-bold" style="color:var(--bs-orange)">{{ matchStates[srv.id].score_t }}</span>
									</template>
									<span v-else class="text-inverse text-opacity-25">—</span>
								</td>
								<td class="text-inverse text-opacity-75">{{ srv.online ? srv.map : '—' }}</td>
								<td>
									<div v-if="srv.online" class="d-flex align-items-center gap-2" style="min-width:100px">
										<span class="text-inverse small">{{ srv.players }}/{{ srv.max_players }}</span>
										<div class="progress flex-grow-1" style="height:4px">
											<div class="progress-bar bg-theme" :style="{ width: srv.max_players > 0 ? (srv.players / srv.max_players * 100) + '%' : '0%' }"></div>
										</div>
									</div>
									<span v-else class="text-inverse text-opacity-50">—</span>
								</td>
								<td class="text-inverse text-opacity-75">{{ srv.online ? srv.ping_ms + ' ms' : '—' }}</td>
								<td>
									<span v-if="srv.last_log_at" :title="new Date(srv.last_log_at).toLocaleString('fr-FR')"
										:class="lastLogClass(srv.last_log_at)" class="small">
										{{ lastLogLabel(srv.last_log_at) }}
									</span>
									<span v-else class="text-inverse text-opacity-25">—</span>
								</td>
								<td class="text-end pe-3" @click.stop>
									<button class="btn btn-outline-danger btn-sm" @click="removeServer(srv)" title="Retirer">
										<i class="fa fa-trash"></i>
									</button>
								</td>
							</tr>

							<!-- Ligne détail (accordéon) -->
							<tr v-if="openServer === srv.id">
								<td colspan="10" class="p-0">
									<div class="px-4 py-3 border-top border-light border-opacity-10">
										<div class="row g-4">

											<!-- Match en cours + Actions -->
											<div class="col-12">
												<p class="text-inverse text-opacity-50 text-uppercase fw-semibold text-center mb-3" style="font-size:.65rem;letter-spacing:.1em">
													<i class="fa fa-gamepad me-1"></i>Match en cours
												</p>

												<template v-if="srv.online && matchStates[srv.id]">

													<!-- Score -->
													<div v-if="matchStates[srv.id].phase !== 'idle'" class="d-flex align-items-center justify-content-center gap-4 mb-4">
														<div class="text-center">
															<div class="fw-bold fs-1 lh-1" style="color:var(--bs-cyan)">{{ matchStates[srv.id].score_ct }}</div>
															<div style="color:var(--bs-cyan);opacity:.5;font-size:.65rem;text-transform:uppercase;letter-spacing:.08em">CT</div>
														</div>
														<span class="text-inverse text-opacity-15 fs-3">:</span>
														<div class="text-center">
															<div class="fw-bold fs-1 lh-1" style="color:var(--bs-orange)">{{ matchStates[srv.id].score_t }}</div>
															<div style="color:var(--bs-orange);opacity:.5;font-size:.65rem;text-transform:uppercase;letter-spacing:.08em">T</div>
														</div>
													</div>

													<!-- Wizard phases -->
													<div class="nav-wizards-container mb-4">
														<nav class="nav nav-wizards-3">
															<div v-for="step in matchSteps(matchStates[srv.id])" :key="step.key" class="nav-item col">
																<a class="nav-link" :class="step.state">
																	<div class="nav-dot"></div>
																	<div class="nav-title">{{ step.label }}</div>
																	<div class="nav-text">{{ step.detail }}</div>
																</a>
															</div>
														</nav>
													</div>

													<template v-if="Object.keys(matchStates[srv.id].players ?? {}).length">
														<div class="table-responsive">
															<table class="table table-sm table-hover mb-0">
																<thead>
																	<tr>
																		<th scope="col">Joueur</th>
																		<th scope="col" class="text-end">K</th>
																		<th scope="col" class="text-end">D</th>
																		<th scope="col" class="text-end">A</th>
																		<th scope="col" class="text-end">DMG</th>
																		<th scope="col" class="text-end">ADR</th>
																		<th scope="col" class="text-end">$</th>
																		<th scope="col" class="text-end">MVP</th>
																	</tr>
																</thead>
																<tbody>
																	<template v-for="[teamKey, teamLabel, teamColor] in [['CT','CT','cyan'],['TERRORIST','T','orange']]" :key="teamKey">
																		<template v-if="playersForTeam(matchStates[srv.id], teamKey).length">
																			<tr>
																				<td colspan="8" class="py-1 px-2" :style="`color:var(--bs-${teamColor});font-size:.62rem;text-transform:uppercase;letter-spacing:.08em;font-weight:700;opacity:.8;background:rgba(var(--bs-${teamColor}-rgb),.05)`">
																				{{ teamLabel }}
																				</td>
																			</tr>
																			<tr v-for="p in playersForTeam(matchStates[srv.id], teamKey)" :key="p.steamid">
																				<td style="max-width:160px">
																					<div class="d-flex align-items-center gap-2">
																						<img v-if="avatarCache[p.steamid]" :src="avatarCache[p.steamid]" width="20" height="20" style="border-radius:3px;flex-shrink:0" />
																						<div v-else style="width:20px;height:20px;border-radius:3px;flex-shrink:0;background:rgba(255,255,255,.08)"></div>
																						<span class="text-truncate">{{ p.name }}</span>
																					</div>
																				</td>
																				<td class="text-end fw-semibold">{{ p.kills }}</td>
																				<td class="text-end text-inverse text-opacity-50">{{ p.deaths }}</td>
																				<td class="text-end text-inverse text-opacity-50">{{ p.assists }}</td>
																				<td class="text-end text-inverse text-opacity-50">{{ p.dmg || '—' }}</td>
																				<td class="text-end text-inverse text-opacity-50">{{ p.adr || '—' }}</td>
																				<td class="text-end" style="color:var(--bs-teal)">{{ p.money ? '$'+p.money : '—' }}</td>
																				<td class="text-end text-inverse text-opacity-50">{{ p.mvp ? '★'.repeat(p.mvp) : '—' }}</td>
																			</tr>
																		</template>
																	</template>
																</tbody>
															</table>
														</div>
													</template>

													<p v-if="matchStates[srv.id].phase === 'idle'" class="text-inverse text-opacity-25 small mb-0">
														Aucune partie en cours.
													</p>
												</template>

												<p v-else-if="!srv.online" class="text-inverse text-opacity-50 small mb-0">
													<i class="fa fa-circle-exclamation me-1 text-danger"></i>Serveur injoignable.
												</p>
												<p v-else class="text-inverse text-opacity-25 small mb-0">Aucune donnée disponible.</p>

												<!-- Actions -->
												<div class="border-top border-light border-opacity-10 pt-3 mt-3">
													<p class="text-inverse text-opacity-50 text-uppercase fw-semibold mb-3" style="font-size:.65rem;letter-spacing:.1em">
														<i class="fa fa-sliders me-1"></i>Actions
													</p>

													<div v-if="srv.online" class="row align-items-center mb-2">
														<label class="col-sm-3 col-form-label col-form-label-sm">Changer la map</label>
														<div class="col-sm-9 d-flex gap-2">
															<select v-model="mapSelections[srv.id]" class="form-select form-select-sm" style="max-width:220px">
																<option value="" disabled selected>Sélectionner…</option>
																<optgroup label="Active Duty">
																	<option v-for="m in cs2Maps" :key="m" :value="m">{{ m }}</option>
																</optgroup>
															</select>
															<button
																class="btn btn-outline-theme btn-sm"
																:disabled="!mapSelections[srv.id] || changingMap[srv.id]"
																@click="changeMap(srv.id)"
															>
																<span v-if="changingMap[srv.id]" class="spinner-border spinner-border-sm"></span>
																<i v-else class="fa fa-arrow-right"></i>
															</button>
														</div>
													</div>

													<div v-if="srv.online" class="row align-items-center mb-2">
														<label class="col-sm-3 col-form-label col-form-label-sm">Configurer</label>
														<div class="col-sm-9">
															<div class="dropdown">
																<button
																	class="btn btn-outline-secondary btn-sm dropdown-toggle"
																	:disabled="pushingCfg[srv.id]"
																	data-bs-toggle="dropdown"
																>
																	<span v-if="pushingCfg[srv.id]" class="spinner-border spinner-border-sm me-1"></span>
																	<i v-else class="fa fa-upload me-1"></i>Pousser une config
																</button>
																<ul class="dropdown-menu">
																	<li>
																		<a class="dropdown-item" href="#" @click.prevent="pushCfg(srv, 'server_init')">
																			<i class="fa fa-file-code me-2 text-inverse text-opacity-50"></i>Default (init serveur)
																		</a>
																	</li>
																	<li v-if="profiles.length"><hr class="dropdown-divider" /></li>
																	<li v-for="p in profiles" :key="p.id">
																		<a class="dropdown-item" href="#" @click.prevent="pushCfg(srv, p.id)">
																			<i class="fa fa-sliders me-2 text-inverse text-opacity-50"></i>{{ p.name }}
																			<span v-if="p.tags?.length" class="text-inverse text-opacity-25 small ms-1">{{ p.tags.join(", ") }}</span>
																		</a>
																	</li>
																</ul>
															</div>
														</div>
													</div>

												<div class="row align-items-center">
														<label class="col-sm-3 col-form-label col-form-label-sm text-danger">Retirer le serveur</label>
														<div class="col-sm-9">
															<button class="btn btn-outline-danger btn-sm" @click="removeServer(srv)">
																<i class="fa fa-trash me-1"></i>Retirer
															</button>
														</div>
													</div>
												</div>
											</div>

										</div>
									</div>
								</td>
							</tr>

						</template>
					</tbody>
				</table>
			</div>
		</card>
	</template>
</template>

<style scoped>
.transition-transform {
	transition: transform .2s ease;
}
</style>
