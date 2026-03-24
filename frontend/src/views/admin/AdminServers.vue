<script>
import config from '@/config/matchmaking'
import { useAuthStore } from '@/stores/auth'

export default {
	data() {
		return {
			activeTab: 'manage',
			servers: [],
			matchStates: {},
			avatarCache: {},
			loading: true,
			newAddr: '',
			newRcon: '',
			adding: false,
			addError: '',
			mapSelections: {},
			changingMap: {},
			matchPollInterval: null,
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
		unmanagedServers() {
			return (this.servers || []).filter(s => !s.managed)
		},
	},
	mounted() {
		this.fetchServers()
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
					fetch(`${config.api.baseUrl}/servers/${encodeURIComponent(s.addr)}/match`, { headers: this.authHeaders() })
						.then(r => r.json())
						.then(state => ({ addr: s.addr, state }))
						.catch(() => null)
				)
			)
			const map = {}
			for (const r of results) {
				if (r) {
					map[r.addr] = r.state
					this.prefetchAvatars(r.state)
				}
			}
			this.matchStates = map
		},
		prefetchAvatars(state) {
			for (const p of Object.values(state.players ?? {})) {
				if (p.steamid && !this.avatarCache[p.steamid]) {
					this.avatarCache[p.steamid] = ''
					fetch(`${config.api.baseUrl}/profile/${encodeURIComponent(p.steamid)}`, { headers: this.authHeaders() })
						.then(r => r.json())
						.then(profile => { this.avatarCache[p.steamid] = profile.avatar || '' })
						.catch(() => {})
				}
			}
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
		async addServer() {
			const target = this.newAddr.trim()
			if (!target) return
			this.adding = true
			this.addError = ''
			try {
				const res = await fetch(`${config.api.baseUrl}/servers`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ addr: target, rcon: this.newRcon.trim() }),
				})
				if (res.ok) {
					this.newAddr = ''
					this.newRcon = ''
					await this.fetchServers()
					this.activeTab = 'manage'
				} else {
					const data = await res.json()
					this.addError = data.error || 'Erreur inconnue'
				}
			} finally {
				this.adding = false
			}
		},
		async changeMap(addr) {
			const map = this.mapSelections[addr]
			if (!map) return
			this.changingMap[addr] = true
			try {
				await fetch(`${config.api.baseUrl}/servers/${encodeURIComponent(addr)}/map`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ map }),
				})
				setTimeout(() => this.fetchServers(), 3000)
			} finally {
				this.changingMap[addr] = false
			}
		},
		async removeServer(addr) {
			await fetch(`${config.api.baseUrl}/servers/${encodeURIComponent(addr)}`, {
				method: 'DELETE',
				headers: this.authHeaders(),
			})
			await this.fetchServers()
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item active">Serveurs</li>
	</ul>
	<div class="d-flex align-items-center justify-content-between mb-3">
		<h1 class="page-header mb-0">Serveurs CS2</h1>
		<button class="btn btn-outline-theme btn-sm" @click="fetchServers" :disabled="loading">
			<i class="fa fa-rotate-right"></i>
		</button>
	</div>

	<!-- Onglets -->
	<ul class="nav nav-tabs mb-4">
		<li class="nav-item">
			<a class="nav-link" :class="{ active: activeTab === 'manage' }" href="#" @click.prevent="activeTab = 'manage'">
				<i class="fa fa-display me-2"></i>Gérer
				<span v-if="!loading" class="badge bg-inverse bg-opacity-15 text-inverse ms-2">{{ servers.length }}</span>
			</a>
		</li>
		<li class="nav-item">
			<a class="nav-link" :class="{ active: activeTab === 'configure' }" href="#" @click.prevent="activeTab = 'configure'">
				<i class="fa fa-gear me-2"></i>Configurer
			</a>
		</li>
	</ul>

	<div v-if="loading" class="text-center py-5">
		<div class="spinner-border text-theme" role="status"></div>
	</div>

	<!-- Onglet Gérer -->
	<template v-else-if="activeTab === 'manage'">
		<div v-if="servers.length === 0" class="alert alert-warning">
			Aucun serveur détecté. Ajoutez-en un depuis l'onglet <strong>Configurer</strong>.
		</div>

		<div v-else class="row g-3">
			<div v-for="srv in servers" :key="srv.addr" class="col-12 col-lg-6 col-xl-4">
				<div class="card h-100" :class="srv.online ? 'border-0' : 'border border-danger border-opacity-25'">
					<div class="card-body">

						<!-- Header -->
						<div class="d-flex align-items-center justify-content-between mb-3">
							<div>
								<span class="fw-bold text-inverse d-block text-truncate" style="max-width:180px" :title="srv.name">
									{{ srv.online ? srv.name : srv.addr }}
								</span>
								<span class="text-inverse text-opacity-25 small">{{ srv.addr }}</span>
							</div>
							<span class="badge" :class="srv.online ? (srv.managed ? 'bg-theme text-theme-900' : 'bg-success bg-opacity-20 text-success') : 'bg-danger bg-opacity-25 text-danger'">
								{{ srv.online ? (srv.managed ? 'Géré' : 'Détecté') : 'Hors ligne' }}
							</span>
						</div>

						<template v-if="srv.online">
							<!-- Infos réseau -->
							<div class="d-flex align-items-center gap-3 text-inverse text-opacity-75 small mb-3">
								<span><i class="fa fa-map me-1 text-theme"></i>{{ srv.map }}</span>
								<span><i class="fa fa-wifi me-1 text-theme"></i>{{ srv.ping_ms }} ms</span>
							</div>

							<!-- Joueurs -->
							<div class="d-flex align-items-center mb-3">
								<span class="text-inverse fw-semibold">{{ srv.players }}</span>
								<span class="text-inverse text-opacity-50 me-3"> / {{ srv.max_players }}</span>
								<div class="progress flex-grow-1" style="height: 5px">
									<div class="progress-bar bg-theme" :style="{ width: srv.max_players > 0 ? (srv.players / srv.max_players * 100) + '%' : '0%' }"></div>
								</div>
								<span v-if="srv.bots > 0" class="text-inverse text-opacity-25 small ms-2">{{ srv.bots }} bots</span>
							</div>

							<!-- État du match -->
							<template v-if="srv.managed && matchStates[srv.addr]">
								<div class="border-top border-light border-opacity-10 pt-3 mt-1 mb-3">
									<div class="d-flex align-items-center justify-content-between mb-2 position-relative">
										<span class="small fw-semibold" :class="phaseClass(matchStates[srv.addr].phase)">
											{{ phaseLabel(matchStates[srv.addr].phase) }}
										</span>
										<span v-if="matchStates[srv.addr].map" class="small text-inverse text-opacity-50 position-absolute start-50 translate-middle-x">
											<i class="fa fa-map fa-xs me-1"></i>{{ matchStates[srv.addr].map }}
										</span>
										<span v-if="matchStates[srv.addr].round > 0" class="small text-inverse text-opacity-50">
											Round {{ matchStates[srv.addr].round }}
										</span>
									</div>

									<div v-if="matchStates[srv.addr].phase !== 'idle'" class="d-flex align-items-center justify-content-center gap-3 mb-3">
										<div class="text-center">
											<div class="fw-bold fs-3 lh-1" style="color: var(--bs-cyan)">{{ matchStates[srv.addr].score_ct }}</div>
											<div style="color: var(--bs-cyan); opacity:.6; font-size:.65rem; text-transform:uppercase; letter-spacing:.05em">CT</div>
										</div>
										<span class="text-inverse text-opacity-25 fs-5">:</span>
										<div class="text-center">
											<div class="fw-bold fs-3 lh-1" style="color: var(--bs-orange)">{{ matchStates[srv.addr].score_t }}</div>
											<div style="color: var(--bs-orange); opacity:.6; font-size:.65rem; text-transform:uppercase; letter-spacing:.05em">T</div>
										</div>
									</div>

									<template v-if="Object.keys(matchStates[srv.addr].players ?? {}).length">
										<template v-for="[teamKey, teamLabel, teamColor] in [['CT','CT','cyan'],['TERRORIST','T','orange']]" :key="teamKey">
											<template v-if="playersForTeam(matchStates[srv.addr], teamKey).length">
												<div class="d-flex align-items-center gap-2 mb-1" :style="`color: var(--bs-${teamColor})`">
													<span style="font-size:.65rem; text-transform:uppercase; letter-spacing:.06em; font-weight:600; opacity:.8">{{ teamLabel }}</span>
													<div class="flex-grow-1 border-top" :style="`border-color: var(--bs-${teamColor}) !important; opacity:.25`"></div>
												</div>
												<table class="w-100 mb-3" style="font-size:.72rem; border-collapse:collapse">
													<thead>
														<tr class="text-inverse text-opacity-25" style="font-size:.63rem">
															<th class="fw-normal pb-1 text-start">Joueur</th>
															<th class="fw-normal pb-1 text-end">K</th>
															<th class="fw-normal pb-1 text-end">D</th>
															<th class="fw-normal pb-1 text-end">A</th>
															<th class="fw-normal pb-1 text-end">DMG</th>
															<th class="fw-normal pb-1 text-end">ADR</th>
															<th class="fw-normal pb-1 text-end">$</th>
															<th class="fw-normal pb-1 text-end">MVP</th>
														</tr>
													</thead>
													<tbody>
														<tr v-for="p in playersForTeam(matchStates[srv.addr], teamKey)" :key="p.steamid" class="text-inverse">
															<td class="py-px" style="max-width:110px">
																<div class="d-flex align-items-center gap-1">
																	<img v-if="avatarCache[p.steamid]" :src="avatarCache[p.steamid]" width="18" height="18" style="border-radius:3px; flex-shrink:0" />
																	<div v-else style="width:18px; height:18px; border-radius:3px; flex-shrink:0; background:rgba(255,255,255,.1)"></div>
																	<span class="text-truncate" style="opacity:.8">{{ p.name }}</span>
																</div>
															</td>
															<td class="text-end py-px">{{ p.kills }}</td>
															<td class="text-end py-px" style="opacity:.5">{{ p.deaths }}</td>
															<td class="text-end py-px" style="opacity:.5">{{ p.assists }}</td>
															<td class="text-end py-px" style="opacity:.5">{{ p.dmg || '—' }}</td>
															<td class="text-end py-px" style="opacity:.5">{{ p.adr || '—' }}</td>
															<td class="text-end py-px" style="color: var(--bs-teal)">{{ p.money ? '$' + p.money : '—' }}</td>
															<td class="text-end py-px" style="opacity:.5">{{ p.mvp ? '★'.repeat(p.mvp) : '—' }}</td>
														</tr>
													</tbody>
												</table>
											</template>
										</template>
									</template>
								</div>
							</template>

							<!-- Actions -->
							<div v-if="srv.managed" class="d-flex gap-2 mb-2">
								<select v-model="mapSelections[srv.addr]" class="form-select form-select-sm flex-grow-1">
									<option value="" disabled selected>Changer la map…</option>
									<optgroup label="Active Duty">
										<option v-for="m in cs2Maps" :key="m" :value="m">{{ m }}</option>
									</optgroup>
								</select>
								<button
									class="btn btn-outline-theme btn-sm"
									:disabled="!mapSelections[srv.addr] || changingMap[srv.addr]"
									@click="changeMap(srv.addr)"
								>
									<span v-if="changingMap[srv.addr]" class="spinner-border spinner-border-sm"></span>
									<i v-else class="fa fa-arrow-right"></i>
								</button>
							</div>

							<div class="d-flex gap-2">
								<a :href="`/connect/${srv.addr}`" class="btn btn-outline-theme btn-sm flex-grow-1">
									<i class="fa fa-play me-1"></i>Rejoindre
								</a>
								<button v-if="!srv.managed" class="btn btn-outline-warning btn-sm" @click="activeTab = 'configure'; newAddr = srv.addr" title="Configurer RCON">
									<i class="fa fa-key"></i>
								</button>
								<button v-else class="btn btn-outline-danger btn-sm" @click="removeServer(srv.addr)" title="Retirer">
									<i class="fa fa-trash"></i>
								</button>
							</div>
						</template>

						<div v-else class="d-flex justify-content-between align-items-center">
							<span class="text-inverse text-opacity-50 small">{{ srv.addr }}</span>
							<button class="btn btn-outline-danger btn-sm" @click="removeServer(srv.addr)" title="Supprimer">
								<i class="fa fa-trash"></i>
							</button>
						</div>

					</div>
				</div>
			</div>
		</div>
	</template>

	<!-- Onglet Configurer -->
	<template v-else-if="activeTab === 'configure'">
		<div class="row g-4">

			<!-- Ajouter manuellement -->
			<div class="col-12 col-md-6">
				<div class="card">
					<div class="card-header fw-semibold text-inverse">
						<i class="fa fa-plus me-2 text-theme"></i>Ajouter un serveur
					</div>
					<div class="card-body">
						<p class="text-inverse text-opacity-50 small mb-3">
							Renseignez l'adresse IP et le mot de passe RCON du serveur CS2. Le backend testera la connexion avant d'enregistrer.
						</p>
						<div class="mb-3">
							<label class="form-label text-inverse text-opacity-75 small">Adresse</label>
							<input
								v-model="newAddr"
								type="text"
								class="form-control"
								placeholder="192.168.6.6:27015"
								@keyup.enter="addServer()"
							/>
						</div>
						<div class="mb-3">
							<label class="form-label text-inverse text-opacity-75 small">Mot de passe RCON</label>
							<input
								v-model="newRcon"
								type="password"
								class="form-control"
								placeholder="••••••••"
								@keyup.enter="addServer()"
							/>
						</div>
						<div v-if="addError" class="alert alert-danger py-2 mb-3">
							<i class="fa fa-triangle-exclamation me-2"></i>{{ addError }}
						</div>
						<button class="btn btn-theme w-100" @click="addServer()" :disabled="adding || !newAddr.trim()">
							<span v-if="adding" class="spinner-border spinner-border-sm me-2"></span>
							<i v-else class="fa fa-plus me-2"></i>Ajouter et configurer les logs
						</button>
					</div>
				</div>
			</div>

			<!-- Serveurs détectés sur le LAN -->
			<div class="col-12 col-md-6">
				<div class="card">
					<div class="card-header fw-semibold text-inverse d-flex align-items-center justify-content-between">
						<span><i class="fa fa-network-wired me-2 text-theme"></i>Détectés sur le LAN</span>
						<button class="btn btn-outline-theme btn-sm" @click="fetchServers" :disabled="loading">
							<i class="fa fa-rotate-right"></i>
						</button>
					</div>
					<div class="card-body p-0">
						<div v-if="unmanagedServers.length === 0" class="px-3 py-4 text-center text-inverse text-opacity-50 small">
							Aucun serveur non configuré détecté sur le réseau.
						</div>
						<ul v-else class="list-group list-group-flush">
							<li v-for="srv in unmanagedServers" :key="srv.addr" class="list-group-item bg-transparent d-flex align-items-center justify-content-between py-2 px-3">
								<div>
									<span class="text-inverse fw-semibold small d-block">{{ srv.online ? srv.name : srv.addr }}</span>
									<span class="text-inverse text-opacity-50" style="font-size:.7rem">
										{{ srv.addr }}
										<span v-if="srv.online"> · {{ srv.map }} · {{ srv.players }}/{{ srv.max_players }} joueurs</span>
									</span>
								</div>
								<button class="btn btn-outline-theme btn-sm" @click="newAddr = srv.addr" title="Utiliser cette adresse">
									<i class="fa fa-arrow-up-right-from-square"></i>
								</button>
							</li>
						</ul>
					</div>
				</div>
			</div>

		</div>
	</template>
</template>
