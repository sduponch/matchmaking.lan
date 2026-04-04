<script>
import config from '@/config/matchmaking'
import { mapDisplayName, mapThumbnail } from '@/config/maps'
import { useAuthStore } from '@/stores/auth'

const STATUS_LABEL = { scheduled: 'Planifiée', live: 'En cours', completed: 'Terminée' }
const STATUS_CLASS = { scheduled: 'text-inverse text-opacity-50', live: 'text-success', completed: 'text-theme' }
const FORMAT_LABEL = { bo1: 'BO1', bo3: 'BO3', bo5: 'BO5' }

const GAME_MODES = [
	{ value: 'defuse',     label: 'Défuse',          prefix: 'de_', icon: 'fa-bomb' },
	{ value: 'casual',     label: 'Occasionnel',     prefix: 'de_', icon: 'fa-people-group' },
	{ value: 'wingman',    label: 'Wingman',         prefix: 'de_', icon: 'fa-user-group' },
	{ value: 'retakes',    label: 'Reprise contrôle',prefix: 'de_', icon: 'fa-rotate-left' },
	{ value: 'hostage',    label: 'Otages',          prefix: 'cs_', icon: 'fa-person-walking-arrow-right' },
	{ value: 'armsrace',   label: 'Arms Race',       prefix: 'ar_', icon: 'fa-gun' },
	{ value: 'deathmatch', label: 'Deathmatch',      prefix: 'dm_', icon: 'fa-skull' },
]


export default {
	data() {
		return {
			encounters: [],
			teams: [],
			servers: [],
			profiles: [],
			loading: true,
			officialMapPool: {},
			openId: null,
			// Create modal
			showCreate: false,
			creating: false,
			newEnc: { team1: '', team2: '', format: 'bo1', game_mode: 'defuse', side_pick: 'knife', launch_mode: 'manual', ready_count: 10, scheduled_at: '', pick_ban: false, map_pool: [], veto_first: 'toss', decider_side: 'pickban', max_rounds: 24, prac: false, overtime: true, ot_start_money: 10000, max_overtimes: 0, tactical_timeouts: 4, tactical_timeout_time: 30, tactical_timeouts_ot: 1, maps: ['', '', '', '', ''] },
			// Start modal
			showStart: false,
			starting: false,
			startTarget: null,
			startForm: { server_id: '', profile_id: '', label: '' },
			// Veto wizard
			showVeto: false,
			vetoTarget: null,
			vetoStep: 'init',      // 'init' | 'veto' | 'sides' | 'done'
			vetoFirstTeam: 0,
			vetoSequence: [],
			vetoCurrentIdx: 0,
			vetoActions: [],
			vetoRemaining: [],
			vetoSides: {},         // mapName -> { choosingTeam, side: 'ct'|'t' }
			vetoDeciderSide: '',   // 'ct' | 't' pour la carte décisive
			// Result override
			showResult: false,
			resultTarget: null,
			resultForm: { map_number: 1, score1: 0, score2: 0 },
			savingResult: false,
		}
	},
	computed: {
		mapsForFormat() {
			return { bo1: 1, bo3: 3, bo5: 5 }[this.newEnc.format] || 1
		},
		managedServers() {
			return (this.servers || []).filter(s => s.managed && s.online)
		},
		compatibleProfiles() {
			const mode = this.startTarget?.game_mode
			if (!mode) return this.profiles
			return (this.profiles || []).filter(p =>
				!p.tags?.length || p.tags.includes(mode)
			)
		},
		currentModePrefix() {
			return GAME_MODES.find(m => m.value === this.newEnc.game_mode)?.prefix ?? ''
		},
		availableMaps() {
			const seen = new Set()
			const maps = []
			const prefix = this.currentModePrefix
			for (const s of this.servers || []) {
				for (const m of s.maps || []) {
					if (!seen.has(m) && (!prefix || m.startsWith(prefix))) {
						seen.add(m); maps.push(m)
					}
				}
			}
			return maps.sort()
		},
		gameModes() { return GAME_MODES },
		pickBanMapPool() {
			const prefix = this.currentModePrefix
			const official = this.officialMapPool[prefix] || []
			const fromServers = this.availableMaps
			const seen = new Set(official)
			const extra = fromServers.filter(m => !seen.has(m))
			return [...official, ...extra]
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
				const [encRes, teamsRes, srvRes, profRes, poolRes] = await Promise.all([
					fetch(`${config.api.baseUrl}/encounters`, { headers: this.authHeaders() }),
					fetch(`${config.api.baseUrl}/teams`, { headers: this.authHeaders() }),
					fetch(`${config.api.baseUrl}/servers`, { headers: this.authHeaders() }),
					fetch(`${config.api.baseUrl}/match-profiles`, { headers: this.authHeaders() }),
					fetch(`${config.api.baseUrl}/map-pool`, { headers: this.authHeaders() }),
				])
				this.encounters = await encRes.json()
				this.teams = await teamsRes.json()
				this.servers = await srvRes.json()
				this.profiles = await profRes.json()
				this.officialMapPool = await poolRes.json()
			} catch {
				this.encounters = []
			} finally {
				this.loading = false
			}
		},
		mapDisplayName(map) { return mapDisplayName(map) },
		mapThumbnail(map) { return mapThumbnail(map) },

		// ── Veto wizard ──────────────────────────────────────────────
		vetoComplete(enc) {
			return enc.pick_ban && enc.maps?.length > 0 && enc.maps[0]?.map !== ''
		},
		openVeto(enc) {
			this.vetoTarget = enc
			this.vetoActions = []
			this.vetoCurrentIdx = 0
			this.vetoRemaining = [...(enc.map_pool || [])]
			this.vetoSides = {}
			this.vetoDeciderSide = ''
			if (enc.veto_first === 'seed') {
				this.vetoFirstTeam = 1
				this.vetoSequence = this.buildVetoSequence(enc.format, enc.map_pool.length, 1)
				this.vetoStep = 'veto'
			} else {
				this.vetoFirstTeam = 0
				this.vetoSequence = []
				this.vetoStep = 'init'
			}
			this.showVeto = true
		},
		startVeto(firstTeam) {
			this.vetoFirstTeam = firstTeam
			this.vetoSequence = this.buildVetoSequence(this.vetoTarget.format, this.vetoTarget.map_pool.length, firstTeam)
			this.vetoStep = 'veto'
		},
		buildVetoSequence(format, poolSize, firstTeam) {
			const mapsNeeded = { bo1: 1, bo3: 3, bo5: 5 }[format] || 1
			const picksNeeded = mapsNeeded - 1
			const bansNeeded = poolSize - mapsNeeded
			const other = t => t === 1 ? 2 : 1
			const seq = []
			let t = firstTeam
			// 2 ouvertures de ban
			const openBans = Math.min(2, bansNeeded)
			for (let i = 0; i < openBans; i++) { seq.push({ team: t, action: 'ban' }); t = other(t) }
			// Picks
			for (let i = 0; i < picksNeeded; i++) { seq.push({ team: t, action: 'pick' }); t = other(t) }
			// Bans restants
			for (let i = 0; i < bansNeeded - openBans; i++) { seq.push({ team: t, action: 'ban' }); t = other(t) }
			return seq
		},
		doVetoAction(map) {
			const step = this.vetoSequence[this.vetoCurrentIdx]
			if (!step) return
			this.vetoActions.push({ ...step, map })
			this.vetoRemaining = this.vetoRemaining.filter(m => m !== map)
			this.vetoCurrentIdx++
			if (this.vetoCurrentIdx >= this.vetoSequence.length) {
				// Veto terminé — préparer étape côtés
				const picks = this.vetoActions.filter(a => a.action === 'pick')
				if (picks.length > 0) {
					for (const a of picks) {
						this.vetoSides[a.map] = { choosingTeam: a.team === 1 ? 2 : 1, side: '' }
					}
					this.vetoStep = 'sides'
				} else {
					// BO1 : aller directement à done (si pas de choix côté interactif)
					const ds = this.vetoTarget.decider_side
					if (ds === 'toss' || ds === 'pickban') {
						this.vetoStep = 'sides'
					} else {
						this.vetoStep = 'done'
					}
				}
			}
		},
		vetoSidesDone() {
			// Vérifier que tous les côtés sont choisis
			const picks = this.vetoActions.filter(a => a.action === 'pick')
			if (picks.some(a => !this.vetoSides[a.map]?.side)) return false
			const ds = this.vetoTarget.decider_side
			if ((ds === 'toss' || ds === 'pickban') && !this.vetoDeciderSide) return false
			return true
		},
		confirmSides() {
			this.vetoStep = 'done'
		},
		async finalizeVeto() {
			const maps = []
			let n = 1
			for (const a of this.vetoActions) {
				if (a.action === 'pick') {
					const s = this.vetoSides[a.map]
					// start_side = côté de team1 : si c'est team2 qui choisit et prend 'ct', team1 est 't'
					let startSide = ''
					if (s?.side) {
						startSide = s.choosingTeam === 1 ? s.side : (s.side === 'ct' ? 't' : 'ct')
					}
					maps.push({ number: n++, map: a.map, score1: 0, score2: 0, status: 'pending', start_side: startSide })
				}
			}
			// Carte décisive
			const decider = this.vetoRemaining[0]
			let deciderSide = ''
			if ((this.vetoTarget.decider_side === 'toss' || this.vetoTarget.decider_side === 'pickban') && this.vetoDeciderSide) {
				deciderSide = this.vetoDeciderSide
			}
			maps.push({ number: n, map: decider, score1: 0, score2: 0, status: 'pending', start_side: deciderSide })

			const res = await fetch(`${config.api.baseUrl}/encounters/${this.vetoTarget.id}/maps`, {
				method: 'PUT',
				headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
				body: JSON.stringify(maps),
			})
			const updated = await res.json()
			const idx = this.encounters.findIndex(e => e.id === this.vetoTarget.id)
			if (idx >= 0) this.encounters[idx] = updated
			this.showVeto = false
			this.openStartModal(updated)
		},
		togglePickBan(enabled) {
			this.newEnc.pick_ban = enabled
			if (enabled) {
				const prefix = this.currentModePrefix
				this.newEnc.map_pool = [...(this.officialMapPool[prefix] || [])]
			} else {
				this.newEnc.map_pool = []
			}
		},
		teamName(id) {
			const t = this.teams.find(t => t.id === id)
			return t ? t.name : id
		},
		serverName(id) {
			const s = this.servers.find(s => s.id === id)
			return s ? (s.name || s.addr) : id
		},
		profileName(id) {
			const p = this.profiles.find(p => p.id === id)
			return p ? p.name : id
		},
		statusLabel(s) { return STATUS_LABEL[s] || s },
		statusClass(s) { return STATUS_CLASS[s] || '' },
		formatLabel(f) { return FORMAT_LABEL[f] || f },
		toggle(id) { this.openId = this.openId === id ? null : id },

		// Create
		openCreateModal() {
			this.newEnc = { team1: '', team2: '', format: 'bo1', game_mode: 'defuse', side_pick: 'knife', launch_mode: 'manual', ready_count: 10, scheduled_at: '', pick_ban: false, map_pool: [], veto_first: 'toss', decider_side: 'pickban', max_rounds: 24, prac: false, overtime: true, ot_start_money: 10000, max_overtimes: 0, tactical_timeouts: 4, tactical_timeout_time: 30, tactical_timeouts_ot: 1, maps: ['', '', '', '', ''] }
			this.showCreate = true
		},
		onGameModeChange() {
			this.newEnc.maps = ['', '', '', '', '']
			this.newEnc.map_pool = []
		},
		async createEncounter() {
			if (!this.newEnc.team1 || !this.newEnc.team2 || !this.newEnc.format) return
			this.creating = true
			const maps = this.newEnc.maps
				.slice(0, this.mapsForFormat)
				.map((m, i) => ({ number: i + 1, map: m, score1: 0, score2: 0, status: 'pending' }))
			try {
				const res = await fetch(`${config.api.baseUrl}/encounters`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ team1: this.newEnc.team1, team2: this.newEnc.team2, format: this.newEnc.format, game_mode: this.newEnc.game_mode, side_pick: this.newEnc.side_pick, launch_mode: this.newEnc.launch_mode, ready_count: this.newEnc.launch_mode === 'ready' ? this.newEnc.ready_count : 0, scheduled_at: this.newEnc.launch_mode === 'scheduled' ? this.newEnc.scheduled_at : null, pick_ban: this.newEnc.pick_ban, map_pool: this.newEnc.pick_ban ? this.newEnc.map_pool : [], veto_first: this.newEnc.pick_ban ? this.newEnc.veto_first : null, decider_side: this.newEnc.pick_ban ? this.newEnc.decider_side : null, max_rounds: this.newEnc.max_rounds, prac: this.newEnc.prac, overtime: this.newEnc.overtime, ot_start_money: this.newEnc.overtime ? this.newEnc.ot_start_money : 0, max_overtimes: this.newEnc.overtime ? this.newEnc.max_overtimes : 0, tactical_timeouts: this.newEnc.tactical_timeouts, tactical_timeout_time: this.newEnc.tactical_timeout_time, tactical_timeouts_ot: this.newEnc.overtime ? this.newEnc.tactical_timeouts_ot : 0, maps }),
				})
				const enc = await res.json()
				this.encounters.unshift(enc)
				this.showCreate = false
			} finally {
				this.creating = false
			}
		},

		// Start
		openStartModal(enc) {
			this.startTarget = enc
			this.startForm = { server_id: '', profile_id: '', label: '' }
			this.showStart = true
		},
		async startEncounter() {
			if (!this.startForm.server_id || !this.startForm.profile_id) return
			this.starting = true
			try {
				const res = await fetch(`${config.api.baseUrl}/encounters/${this.startTarget.id}/start`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify(this.startForm),
				})
				const enc = await res.json()
				const idx = this.encounters.findIndex(e => e.id === enc.id)
				if (idx >= 0) this.encounters[idx] = enc
				this.showStart = false
			} finally {
				this.starting = false
			}
		},

		// Reopen
		async reopen(enc) {
			if (!confirm(`Réinitialiser la rencontre "${this.teamName(enc.team1)} vs ${this.teamName(enc.team2)}" ?`)) return
			const res = await fetch(`${config.api.baseUrl}/encounters/${enc.id}/reopen`, {
				method: 'POST', headers: this.authHeaders(),
			})
			const updated = await res.json()
			const idx = this.encounters.findIndex(e => e.id === enc.id)
			if (idx >= 0) this.encounters[idx] = updated
		},

		// Delete
		async deleteEncounter(enc) {
			if (!confirm(`Supprimer la rencontre ?`)) return
			await fetch(`${config.api.baseUrl}/encounters/${enc.id}`, { method: 'DELETE', headers: this.authHeaders() })
			this.encounters = this.encounters.filter(e => e.id !== enc.id)
			if (this.openId === enc.id) this.openId = null
		},

		// Result override
		openResultModal(enc) {
			this.resultTarget = enc
			this.resultForm = { map_number: enc.maps?.find(m => m.status === 'live')?.number || 1, score1: 0, score2: 0 }
			this.showResult = true
		},
		async saveResult() {
			this.savingResult = true
			try {
				const res = await fetch(`${config.api.baseUrl}/encounters/${this.resultTarget.id}/result`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify(this.resultForm),
				})
				const enc = await res.json()
				const idx = this.encounters.findIndex(e => e.id === enc.id)
				if (idx >= 0) this.encounters[idx] = enc
				this.showResult = false
			} finally {
				this.savingResult = false
			}
		},

		mapWinner(enc, side) {
			return (enc.maps || []).filter(m => m.winner === side).length
		},
		winsNeeded(format) {
			return { bo1: 1, bo3: 2, bo5: 3 }[format] || 1
		},
		winnerLabel(enc) {
			if (!enc.winner) return ''
			return enc.winner === 'team1' ? enc.team1_name || enc.team1 : enc.team2_name || enc.team2
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item active">Rencontres</li>
	</ul>

	<div v-if="loading" class="text-center py-5">
		<div class="spinner-border text-theme"></div>
	</div>

	<template v-else>
		<card>
			<card-header class="d-flex align-items-center justify-content-between fw-semibold">
				<div class="d-flex align-items-center gap-2">
					<i class="fa fa-swords text-theme me-1"></i>Rencontres
					<span class="badge bg-inverse bg-opacity-15 text-inverse fw-normal">{{ encounters.length }}</span>
				</div>
				<div class="d-flex gap-2">
					<button class="btn btn-outline-theme btn-sm" @click="fetchAll" :disabled="loading">
						<i class="fa fa-rotate-right"></i>
					</button>
					<button class="btn btn-theme btn-sm" @click="openCreateModal">
						<i class="fa fa-plus me-1"></i>Nouvelle rencontre
					</button>
				</div>
			</card-header>

			<div v-if="encounters.length === 0" class="card-body text-inverse text-opacity-50">
				Aucune rencontre. Créez-en une pour commencer.
			</div>

			<div v-else>
				<template v-for="enc in encounters" :key="enc.id">
					<!-- Row -->
					<div
						class="d-flex align-items-center px-4 py-3 border-bottom border-light border-opacity-10"
						style="cursor:pointer"
						:class="{ 'bg-inverse bg-opacity-5': openId === enc.id }"
						@click="toggle(enc.id)"
					>
						<i class="fa fa-chevron-right fa-xs text-inverse text-opacity-25 me-3 flex-shrink-0 transition-transform"
							:style="openId === enc.id ? 'transform:rotate(90deg)' : ''"></i>

						<!-- Teams + format -->
						<div class="flex-grow-1 d-flex align-items-center gap-3 flex-wrap">
							<span class="fw-semibold">
								{{ enc.team1_name || enc.team1 }}
								<span class="text-inverse text-opacity-25 mx-1">vs</span>
								{{ enc.team2_name || enc.team2 }}
							</span>
							<span class="badge bg-inverse bg-opacity-10 text-inverse fw-normal">{{ formatLabel(enc.format) }}</span>
							<span v-if="enc.game_mode && enc.game_mode !== 'defuse'" class="badge bg-inverse bg-opacity-10 text-inverse fw-normal">
								<i :class="['fa', gameModes.find(m=>m.value===enc.game_mode)?.icon ?? 'fa-gamepad', 'me-1']"></i>{{ gameModes.find(m=>m.value===enc.game_mode)?.label ?? enc.game_mode }}
							</span>

							<!-- Score (if started) -->
							<template v-if="enc.status !== 'scheduled'">
								<span class="fw-bold" style="color:var(--bs-teal)">
									{{ mapWinner(enc, 'team1') }} – {{ mapWinner(enc, 'team2') }}
								</span>
								<span v-if="enc.winner" class="badge bg-success bg-opacity-15 text-success">
									<i class="fa fa-trophy me-1"></i>{{ winnerLabel(enc) }}
								</span>
							</template>

							<!-- Server (if live) -->
							<span v-if="enc.status === 'live' && enc.server_id" class="text-inverse text-opacity-50 small">
								<i class="fa fa-server fa-xs me-1"></i>{{ serverName(enc.server_id) }}
							</span>
						</div>

						<!-- Status + actions -->
						<div class="d-flex align-items-center gap-3" @click.stop>
							<span class="small fw-semibold" :class="statusClass(enc.status)">
								<span v-if="enc.status === 'live'" class="me-1" style="animation:pulse 1.5s infinite">●</span>
								{{ statusLabel(enc.status) }}
							</span>
							<button v-if="enc.status === 'scheduled' && enc.pick_ban && !vetoComplete(enc)" class="btn btn-outline-warning btn-sm" @click="openVeto(enc)" title="Pick &amp; Ban">
								<i class="fa fa-shuffle me-1"></i>Pick &amp; Ban
							</button>
							<button v-if="enc.status === 'scheduled' && (!enc.pick_ban || vetoComplete(enc))" class="btn btn-outline-theme btn-sm" @click="openStartModal(enc)" title="Lancer">
								<i class="fa fa-play me-1"></i>Lancer
							</button>
							<button v-if="enc.status === 'live'" class="btn btn-outline-warning btn-sm" @click="openResultModal(enc)" title="Saisir résultat">
								<i class="fa fa-pen me-1"></i>Résultat
							</button>
							<div class="dropdown">
								<button class="btn btn-outline-secondary btn-sm" data-bs-toggle="dropdown">
									<i class="fa fa-ellipsis-v"></i>
								</button>
								<ul class="dropdown-menu dropdown-menu-end">
									<li v-if="enc.status !== 'scheduled'">
										<a class="dropdown-item" href="#" @click.prevent="reopen(enc)">
											<i class="fa fa-rotate-left me-2 text-warning"></i>Réinitialiser
										</a>
									</li>
									<li v-if="enc.status === 'live'">
										<a class="dropdown-item" href="#" @click.prevent="openResultModal(enc)">
											<i class="fa fa-pen me-2"></i>Saisir résultat
										</a>
									</li>
									<li><hr class="dropdown-divider" /></li>
									<li>
										<a class="dropdown-item text-danger" href="#" @click.prevent="deleteEncounter(enc)">
											<i class="fa fa-trash me-2"></i>Supprimer
										</a>
									</li>
								</ul>
							</div>
						</div>
					</div>

					<!-- Expanded: maps detail -->
					<div v-if="openId === enc.id" class="px-4 py-3 border-bottom border-light border-opacity-10 bg-inverse bg-opacity-3">
						<div class="row g-3">
							<!-- Maps -->
							<div class="col-md-8">
								<p class="text-inverse text-opacity-50 text-uppercase fw-semibold mb-2" style="font-size:.65rem;letter-spacing:.1em">
									<i class="fa fa-map me-1"></i>Maps
								</p>
								<div class="d-flex flex-column gap-2">
									<div v-for="m in enc.maps" :key="m.number"
										class="d-flex align-items-center gap-3 px-3 py-2 rounded"
										:class="{
											'bg-inverse bg-opacity-5': m.status === 'pending',
											'border border-theme border-opacity-25': m.status === 'live',
										}"
									>
										<span class="text-inverse text-opacity-25 small" style="min-width:1.5rem">{{ m.number }}</span>
										<span class="flex-grow-1 fw-semibold">{{ m.map || '—' }}</span>
										<template v-if="m.status === 'completed'">
											<span class="fw-bold" :class="m.winner === 'team1' ? 'text-success' : 'text-inverse text-opacity-50'">{{ m.score1 }}</span>
											<span class="text-inverse text-opacity-25 mx-1">–</span>
											<span class="fw-bold" :class="m.winner === 'team2' ? 'text-success' : 'text-inverse text-opacity-50'">{{ m.score2 }}</span>
											<i class="fa fa-trophy text-warning fa-xs ms-1"></i>
										</template>
										<template v-else-if="m.status === 'live'">
											<span class="badge border border-theme text-theme" style="font-size:.7rem">En cours</span>
										</template>
										<template v-else>
											<span class="text-inverse text-opacity-25 small">À venir</span>
										</template>
									</div>
								</div>
							</div>

							<!-- Infos -->
							<div class="col-md-4">
								<p class="text-inverse text-opacity-50 text-uppercase fw-semibold mb-2" style="font-size:.65rem;letter-spacing:.1em">
									<i class="fa fa-circle-info me-1"></i>Infos
								</p>
								<div class="d-flex flex-column gap-1" style="font-size:.85rem">
									<div v-if="enc.server_id">
										<span class="text-inverse text-opacity-50">Serveur : </span>{{ serverName(enc.server_id) }}
									</div>
									<div v-if="enc.profile_id">
										<span class="text-inverse text-opacity-50">Profil : </span>{{ profileName(enc.profile_id) }}
									</div>
									<div v-if="enc.started_at">
										<span class="text-inverse text-opacity-50">Début : </span>{{ new Date(enc.started_at).toLocaleString('fr-FR') }}
									</div>
									<div v-if="enc.ended_at">
										<span class="text-inverse text-opacity-50">Fin : </span>{{ new Date(enc.ended_at).toLocaleString('fr-FR') }}
									</div>
									<div>
										<span class="text-inverse text-opacity-50">Démo : </span>
										<span :class="enc.demo_status === 'recording' ? 'text-danger' : 'text-inverse text-opacity-50'">
											{{ enc.demo_status === 'recording' ? '● Enregistrement' : enc.demo_status || 'none' }}
										</span>
									</div>
								</div>
							</div>
						</div>
					</div>
				</template>
			</div>
		</card>
	</template>

	<!-- Create modal -->
	<teleport to="body">
		<div v-if="showCreate" class="modal d-block" tabindex="-1" style="background:rgba(0,0,0,.6)">
			<div class="modal-dialog">
				<div class="modal-content">
					<div class="modal-header border-light border-opacity-10">
						<h5 class="modal-title"><i class="fa fa-plus me-2 text-theme"></i>Nouvelle rencontre</h5>
						<button class="btn-close" @click="showCreate = false"></button>
					</div>
					<div class="modal-body">
						<div class="row g-3 mb-3">
							<div class="col-6">
								<label class="form-label">Équipe 1 <span class="text-danger">*</span></label>
								<select v-model="newEnc.team1" class="form-select">
									<option value="" disabled>Sélectionner…</option>
									<option v-for="t in teams" :key="t.id" :value="t.id" :disabled="t.id === newEnc.team2">{{ t.name }}</option>
								</select>
							</div>
							<div class="col-6">
								<label class="form-label">Équipe 2 <span class="text-danger">*</span></label>
								<select v-model="newEnc.team2" class="form-select">
									<option value="" disabled>Sélectionner…</option>
									<option v-for="t in teams" :key="t.id" :value="t.id" :disabled="t.id === newEnc.team1">{{ t.name }}</option>
								</select>
							</div>
						</div>
						<div class="mb-3">
							<label class="form-label">Mode de jeu</label>
							<div class="d-flex gap-2 flex-wrap">
								<button v-for="m in gameModes" :key="m.value"
									class="btn btn-sm flex-fill"
									:class="newEnc.game_mode === m.value ? 'btn-theme' : 'btn-outline-secondary'"
									@click="newEnc.game_mode = m.value; onGameModeChange()">
									<i :class="['fa', m.icon, 'me-1']"></i>{{ m.label }}
								</button>
							</div>
						</div>
						<!-- Format -->
						<div class="modal-section-label">Format</div>
						<div class="mb-3">
							<div class="mb-2 d-flex align-items-center gap-2 flex-wrap">
								<span class="text-inverse text-opacity-50 small">Nombre de manches :</span>
								<input v-model.number="newEnc.max_rounds" type="number" min="6" max="60" step="2"
									class="form-control form-control-sm" style="max-width:70px" />
								<div class="form-check mb-0">
									<input class="form-check-input" type="checkbox" id="enc_prac"
										:checked="!newEnc.prac" @change="newEnc.prac = !$event.target.checked" />
									<label class="form-check-label small" for="enc_prac">Victoire anticipée</label>
								</div>
								<span v-if="!newEnc.prac" class="badge bg-secondary">MR{{ newEnc.max_rounds / 2 }}</span>
							</div>
							<div class="mb-2 d-flex align-items-center gap-2">
								<span class="text-inverse text-opacity-50 small" style="min-width:130px">Pauses tactiques</span>
								<input v-model.number="newEnc.tactical_timeouts" type="number" min="0" max="10"
									class="form-control form-control-sm" style="max-width:60px" />
								<span class="text-inverse text-opacity-50 small">pause{{ newEnc.tactical_timeouts !== 1 ? 's' : '' }}&nbsp;de</span>
								<input v-model.number="newEnc.tactical_timeout_time" type="number" min="10" max="120" step="5"
									class="form-control form-control-sm" style="max-width:60px" />
								<span class="text-inverse text-opacity-50 small">s&nbsp;/&nbsp;équipe</span>
							</div>
							<div class="d-flex gap-2">
								<template v-for="f in ['bo1','bo3','bo5']" :key="f">
									<button class="btn btn-sm flex-fill"
										:class="newEnc.format === f ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.format = f">{{ f.toUpperCase() }}</button>
								</template>
							</div>
						</div>
						<!-- Prolongations -->
						<div class="mb-1 d-flex align-items-center gap-3">
							<div class="form-check mb-0">
								<input class="form-check-input" type="checkbox" v-model="newEnc.overtime" id="enc_overtime" />
								<label class="form-check-label" for="enc_overtime">Prolongations</label>
							</div>
							<span v-if="!newEnc.overtime" class="text-inverse text-opacity-25 small">match nul possible</span>
						</div>
						<div v-if="newEnc.overtime" class="mb-1 d-flex align-items-center gap-2">
							<span class="text-inverse text-opacity-50 small" style="min-width:130px">Prolongations max</span>
							<input v-model.number="newEnc.max_overtimes" type="number" min="0" max="20" step="1"
								class="form-control form-control-sm" style="max-width:60px" />
							<span class="text-inverse text-opacity-50 small">{{ newEnc.max_overtimes === 0 ? '∞ illimité' : (newEnc.max_overtimes === 1 ? 'prolongation' : 'prolongations') }}</span>
						</div>
						<div v-if="newEnc.overtime" class="mb-1 d-flex align-items-center gap-2">
							<span class="text-inverse text-opacity-50 small" style="min-width:130px">Argent de départ</span>
							<input v-model.number="newEnc.ot_start_money" type="number" min="0" max="16000" step="1000"
								class="form-control form-control-sm" style="max-width:90px" />
							<span class="text-inverse text-opacity-50 small">$&nbsp;/&nbsp;joueur</span>
						</div>
						<div v-if="newEnc.overtime" class="mb-2 d-flex align-items-center gap-2">
							<span class="text-inverse text-opacity-50 small" style="min-width:130px">Pauses tactiques (OT)</span>
							<input v-model.number="newEnc.tactical_timeouts_ot" type="number" min="0" max="4" step="1"
								class="form-control form-control-sm" style="max-width:60px" />
							<span class="text-inverse text-opacity-50 small">pause{{ newEnc.tactical_timeouts_ot !== 1 ? 's' : '' }}&nbsp;de</span>
							<input v-model.number="newEnc.tactical_timeout_time" type="number" min="10" max="120" step="5"
								class="form-control form-control-sm" style="max-width:60px" />
							<span class="text-inverse text-opacity-50 small">s&nbsp;/&nbsp;équipe</span>
						</div>
						<!-- Section : Choix de la carte -->
						<div class="modal-section-label">Choix de la carte</div>
						<div class="mb-0">
							<div class="form-check mb-2">
								<input class="form-check-input" type="checkbox" :checked="newEnc.pick_ban" id="enc_pickban"
									@change="togglePickBan($event.target.checked)" />
								<label class="form-check-label" for="enc_pickban">Pick &amp; Ban</label>
							</div>

							<!-- Pick & Ban: pool de cartes -->
							<template v-if="newEnc.pick_ban">
								<label class="form-label form-label-sm">Pool de cartes éligibles</label>
								<div v-if="pickBanMapPool.length === 0" class="form-text text-inverse text-opacity-50 mb-2">
									<i class="fa fa-circle-info me-1"></i>Aucune carte disponible pour ce mode.
								</div>
								<div class="d-flex flex-wrap gap-2">
									<label v-for="m in pickBanMapPool" :key="m"
										class="d-flex align-items-center gap-1 px-2 py-1 rounded border map-pool-item"
										:class="newEnc.map_pool.includes(m) ? 'border-theme bg-theme bg-opacity-10 text-theme' : 'border-light border-opacity-15 text-inverse text-opacity-75'"
										style="cursor:pointer;font-size:.82rem;user-select:none"
										@click="newEnc.map_pool.includes(m) ? newEnc.map_pool.splice(newEnc.map_pool.indexOf(m),1) : newEnc.map_pool.push(m)">
										<i class="fa fa-check fa-xs me-1" :style="newEnc.map_pool.includes(m) ? '' : 'opacity:0'"></i>
										{{ mapDisplayName(m) }}
									</label>
								</div>
								<div class="form-text text-inverse text-opacity-50 mt-1">
									{{ newEnc.map_pool.length }} carte{{ newEnc.map_pool.length !== 1 ? 's' : '' }} sélectionnée{{ newEnc.map_pool.length !== 1 ? 's' : '' }}
									<span v-if="newEnc.map_pool.length > 0" class="ms-2 text-theme" style="cursor:pointer" @click="newEnc.map_pool = []">
										<i class="fa fa-xmark fa-xs me-1"></i>Tout désélectionner
									</span>
								</div>
							</template>

							<!-- Manuel: selects de maps -->
							<template v-else>
								<label class="form-label">
									Maps <span class="text-inverse text-opacity-50 small">(optionnel)</span>
								</label>
								<div class="d-flex flex-column gap-2">
									<div v-for="i in mapsForFormat" :key="i" class="d-flex align-items-center gap-2">
										<span class="text-inverse text-opacity-50 small fw-semibold" style="min-width:1.5rem">{{ i }}</span>
										<template v-if="availableMaps.length > 0">
											<select v-model="newEnc.maps[i-1]" class="form-select form-select-sm">
												<option value="">— TBD —</option>
												<option v-for="m in availableMaps" :key="m" :value="m">{{ m }}</option>
											</select>
										</template>
										<template v-else>
											<input v-model="newEnc.maps[i-1]" class="form-control form-control-sm"
												:placeholder="`Map ${i} (ex: de_mirage)`" />
										</template>
									</div>
								</div>
								<div v-if="availableMaps.length === 0" class="form-text text-inverse text-opacity-50">
									<i class="fa fa-circle-info me-1"></i>Aucun serveur géré — saisissez les noms manuellement.
								</div>
							</template>
						</div>

						<!-- Section : Choix du côté de départ -->
						<div class="modal-section-label">Choix du côté de départ</div>
						<div class="mb-3">
							<!-- Pick & Ban actif : choix pour la carte décisive -->
							<template v-if="newEnc.pick_ban">
								<p class="text-inverse text-opacity-50 mb-2" style="font-size:.82rem">
									<i class="fa fa-circle-info me-1"></i>
									Sur les cartes pickées, l'équipe adverse choisit son camp. Pour la <strong>carte décisive</strong> (restante), choisissez la méthode ci-dessous.
								</p>
								<label class="form-label form-label-sm">Qui commence le veto</label>
								<div class="d-flex gap-2 mb-3">
									<button class="btn btn-sm flex-fill"
										:class="newEnc.veto_first === 'seed' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.veto_first = 'seed'">
										<i class="fa fa-ranking-star me-1"></i>Seed
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.veto_first === 'toss' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.veto_first = 'toss'">
										<i class="fa fa-coins me-1"></i>Aléatoire
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.veto_first === 'chifoumi' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.veto_first = 'chifoumi'">
										<i class="fa fa-trophy me-1"></i>Challenge
									</button>
								</div>
								<label class="form-label form-label-sm">Camp de départ sur la carte décisive</label>
								<div class="d-flex gap-2 flex-wrap">
									<button class="btn btn-sm flex-fill"
										:class="newEnc.decider_side === 'pickban' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.decider_side = 'pickban'">
										<i class="fa fa-shuffle me-1"></i>Pick &amp; Ban
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.decider_side === 'toss' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.decider_side = 'toss'">
										<i class="fa fa-coins me-1"></i>Aléatoire
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.decider_side === 'knife' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.decider_side = 'knife'">
										<i class="fa fa-knife me-1"></i>Knife
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.decider_side === 'vote' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.decider_side = 'vote'">
										<i class="fa fa-hand-point-up me-1"></i>Vote joueurs
									</button>
								</div>
								<div class="form-text text-inverse text-opacity-50">
									<template v-if="newEnc.decider_side === 'pickban'">L'équipe ayant fait le dernier ban choisit son camp de départ sur la carte décisive.</template>
									<template v-else-if="newEnc.decider_side === 'toss'">Un tirage aléatoire détermine le camp de départ sur la carte décisive.</template>
									<template v-else-if="newEnc.decider_side === 'knife'">Un round couteaux détermine le camp de départ sur la carte décisive.</template>
									<template v-else>Les joueurs votent en jeu pour leur camp de départ sur la carte décisive.</template>
								</div>
							</template>
							<!-- Sélection manuelle -->
							<template v-else>
								<div class="d-flex gap-2">
									<button class="btn btn-sm flex-fill"
										:class="newEnc.side_pick === 'knife' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.side_pick = 'knife'">
										<i class="fa fa-knife me-1"></i>Round couteaux
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.side_pick === 'ct' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.side_pick = 'ct'">
										<i class="fa fa-shield me-1"></i>CT
									</button>
									<button class="btn btn-sm flex-fill"
										:class="newEnc.side_pick === 't' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="newEnc.side_pick = 't'">
										<i class="fa fa-bomb me-1"></i>T
									</button>
								</div>
								<div class="form-text text-inverse text-opacity-50">
									<template v-if="newEnc.side_pick === 'knife'">Un round couteaux détermine quel camp commence</template>
									<template v-else-if="newEnc.side_pick === 'ct'">
										<span class="fw-semibold text-inverse">{{ newEnc.team1 ? teamName(newEnc.team1) : 'Équipe 1' }}</span> commence CT (Défense) —
										<span class="fw-semibold text-inverse">{{ newEnc.team2 ? teamName(newEnc.team2) : 'Équipe 2' }}</span> commence T (Attaque)
									</template>
									<template v-else>
										<span class="fw-semibold text-inverse">{{ newEnc.team1 ? teamName(newEnc.team1) : 'Équipe 1' }}</span> commence T (Attaque) —
										<span class="fw-semibold text-inverse">{{ newEnc.team2 ? teamName(newEnc.team2) : 'Équipe 2' }}</span> commence CT (Défense)
									</template>
								</div>
							</template>
						</div>

						<!-- Section : Mode de lancement -->
						<div class="modal-section-label">Mode de lancement de la rencontre</div>
						<div class="mb-2">
							<div class="d-flex gap-2 mb-2">
								<button class="btn btn-sm flex-fill"
									:class="newEnc.launch_mode === 'manual' ? 'btn-theme' : 'btn-outline-secondary'"
									@click="newEnc.launch_mode = 'manual'">
									<i class="fa fa-hand-pointer me-1"></i>Manuel
								</button>
								<button class="btn btn-sm flex-fill"
									:class="newEnc.launch_mode === 'scheduled' ? 'btn-theme' : 'btn-outline-secondary'"
									@click="newEnc.launch_mode = 'scheduled'">
									<i class="fa fa-calendar me-1"></i>Planifié
								</button>
								<button class="btn btn-sm flex-fill"
									:class="newEnc.launch_mode === 'ready' ? 'btn-theme' : 'btn-outline-secondary'"
									@click="newEnc.launch_mode = 'ready'">
									<i class="fa fa-circle-check me-1"></i>Ready
								</button>
							</div>
							<div v-if="newEnc.launch_mode === 'manual'" class="form-text text-inverse text-opacity-50">
								L'admin lance la rencontre manuellement depuis cette interface.
							</div>
							<div v-else-if="newEnc.launch_mode === 'scheduled'" class="mt-1">
								<label class="form-label form-label-sm">Date et heure de départ</label>
								<input v-model="newEnc.scheduled_at" type="datetime-local" class="form-control form-control-sm" style="max-width:220px" />
							</div>
							<div v-else-if="newEnc.launch_mode === 'ready'" class="mt-1">
								<label class="form-label form-label-sm">Joueurs requis pour démarrer</label>
								<input v-model.number="newEnc.ready_count" type="number" min="1" max="10" class="form-control form-control-sm" style="max-width:100px" />
								<div class="form-text text-inverse text-opacity-50">Les joueurs tapent <code>!ready</code> en jeu pour signaler leur présence.</div>
							</div>
						</div>

					</div>
					<div class="modal-footer border-light border-opacity-10">
						<button class="btn btn-outline-secondary btn-sm" @click="showCreate = false">Annuler</button>
						<button class="btn btn-theme btn-sm" @click="createEncounter"
							:disabled="creating || !newEnc.team1 || !newEnc.team2 || newEnc.team1 === newEnc.team2">
							<span v-if="creating" class="spinner-border spinner-border-sm me-1"></span>
							<i v-else class="fa fa-plus me-1"></i>Créer
						</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Start modal -->
		<!-- ── Veto wizard ──────────────────────────────────────────── -->
		<div v-if="showVeto && vetoTarget" class="modal d-block" tabindex="-1" style="background:rgba(0,0,0,.75);z-index:1100">
			<div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable">
				<div class="modal-content">
					<div class="modal-header border-light border-opacity-10">
						<h5 class="modal-title">
							<i class="fa fa-shuffle me-2 text-theme"></i>Pick &amp; Ban
							<small class="text-inverse text-opacity-50 ms-2">{{ teamName(vetoTarget.team1) }} vs {{ teamName(vetoTarget.team2) }}</small>
						</h5>
						<button class="btn-close" @click="showVeto = false"></button>
					</div>
					<div class="modal-body">

						<!-- STEP: init — qui commence -->
						<template v-if="vetoStep === 'init'">
							<p class="text-inverse text-opacity-75 mb-3">
								<template v-if="vetoTarget.veto_first === 'toss'">Tirage au sort — quelle équipe commence le veto ?</template>
								<template v-else>Challenge (chifoumi) — quelle équipe a gagné et commence le veto ?</template>
							</p>
							<div class="d-flex gap-3">
								<button class="btn btn-outline-theme flex-fill py-3" @click="startVeto(1)">
									<i class="fa fa-flag me-2"></i>{{ teamName(vetoTarget.team1) }}
								</button>
								<button class="btn btn-outline-theme flex-fill py-3" @click="startVeto(2)">
									<i class="fa fa-flag me-2"></i>{{ teamName(vetoTarget.team2) }}
								</button>
							</div>
						</template>

						<!-- STEP: veto — ban/pick -->
						<template v-if="vetoStep === 'veto'">
							<!-- Indicateur d'action courante -->
							<div v-if="vetoCurrentIdx < vetoSequence.length" class="d-flex align-items-center gap-3 mb-3 p-3 rounded" style="background:rgba(255,255,255,.05)">
								<span class="badge fs-6" :class="vetoSequence[vetoCurrentIdx].action === 'ban' ? 'bg-danger' : 'bg-success'">
									{{ vetoSequence[vetoCurrentIdx].action === 'ban' ? 'BAN' : 'PICK' }}
								</span>
								<span class="fw-semibold">
									{{ vetoSequence[vetoCurrentIdx].team === 1 ? teamName(vetoTarget.team1) : teamName(vetoTarget.team2) }}
								</span>
								<span class="text-inverse text-opacity-50 small ms-auto">
									{{ vetoCurrentIdx + 1 }} / {{ vetoSequence.length }}
								</span>
							</div>

							<!-- Map pool -->
							<div class="d-flex flex-wrap gap-2 mb-3">
								<div v-for="m in vetoTarget.map_pool" :key="m"
									class="veto-card"
									:class="{
										'veto-card--banned':  vetoActions.some(a => a.map === m && a.action === 'ban'),
										'veto-card--picked':  vetoActions.some(a => a.map === m && a.action === 'pick'),
										'veto-card--active':  vetoRemaining.includes(m) && vetoCurrentIdx < vetoSequence.length,
									}"
									@click="vetoRemaining.includes(m) && vetoCurrentIdx < vetoSequence.length ? doVetoAction(m) : null">
									<div class="veto-card__thumb">
										<img :src="mapThumbnail(m)" :alt="mapDisplayName(m)" @error="$event.target.style.display='none'" />
										<div class="veto-card__overlay">
											<template v-if="vetoActions.some(a => a.map === m && a.action === 'ban')">
												<i class="fa fa-xmark fa-2x text-danger"></i>
											</template>
											<template v-else-if="vetoActions.some(a => a.map === m && a.action === 'pick')">
												<i class="fa fa-check fa-2x text-success"></i>
											</template>
										</div>
									</div>
									<div class="veto-card__name">
										<template v-if="vetoActions.some(a => a.map === m && a.action === 'ban')">
											<span class="text-danger">{{ mapDisplayName(m) }}</span>
										</template>
										<template v-else-if="vetoActions.some(a => a.map === m && a.action === 'pick')">
											<span class="text-success">{{ mapDisplayName(m) }}</span>
											<small class="d-block text-inverse text-opacity-50" style="font-size:.6rem">
												Choisi par {{ vetoActions.find(a => a.map === m).team === 1 ? teamName(vetoTarget.team1) : teamName(vetoTarget.team2) }}
											</small>
										</template>
										<template v-else-if="vetoRemaining.length === 1 && vetoRemaining[0] === m">
											<span class="text-warning">{{ mapDisplayName(m) }}</span>
											<small class="d-block text-inverse text-opacity-50" style="font-size:.6rem">Décisive</small>
										</template>
										<template v-else>{{ mapDisplayName(m) }}</template>
									</div>
								</div>
							</div>

							<!-- Historique -->
							<div v-if="vetoActions.length" class="d-flex flex-wrap gap-1">
								<span v-for="(a, i) in vetoActions" :key="i"
									class="badge"
									:class="a.action === 'ban' ? 'bg-danger bg-opacity-75' : 'bg-success bg-opacity-75'">
									{{ a.team === 1 ? teamName(vetoTarget.team1) : teamName(vetoTarget.team2) }}
									{{ a.action === 'ban' ? 'ban' : 'pick' }}
									{{ mapDisplayName(a.map) }}
								</span>
							</div>
						</template>

						<!-- STEP: sides — choix des côtés -->
						<template v-if="vetoStep === 'sides'">
							<p class="text-inverse text-opacity-75 mb-3">Choisissez le camp de départ pour chaque carte.</p>

							<!-- Cartes pickées -->
							<div v-for="a in vetoActions.filter(x => x.action === 'pick')" :key="a.map" class="mb-3 p-3 rounded" style="background:rgba(255,255,255,.05)">
								<div class="d-flex align-items-center gap-2 mb-2">
									<span class="fw-semibold">{{ mapDisplayName(a.map) }}</span>
									<span class="text-inverse text-opacity-50 small">— choisit par {{ a.team === 1 ? teamName(vetoTarget.team1) : teamName(vetoTarget.team2) }}</span>
								</div>
								<div class="small text-inverse text-opacity-50 mb-2">
									{{ a.team === 1 ? teamName(vetoTarget.team2) : teamName(vetoTarget.team1) }} choisit son camp :
								</div>
								<div class="d-flex gap-2">
									<button class="btn btn-sm flex-fill"
										:class="vetoSides[a.map]?.side === 'ct' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="vetoSides[a.map] = { choosingTeam: a.team === 1 ? 2 : 1, side: 'ct' }">
										<i class="fa fa-shield me-1"></i>CT (Défense)
									</button>
									<button class="btn btn-sm flex-fill"
										:class="vetoSides[a.map]?.side === 't' ? 'btn-warning' : 'btn-outline-secondary'"
										@click="vetoSides[a.map] = { choosingTeam: a.team === 1 ? 2 : 1, side: 't' }">
										<i class="fa fa-bomb me-1"></i>T (Attaque)
									</button>
								</div>
							</div>

							<!-- Carte décisive (si côté interactif) -->
							<div v-if="vetoTarget.decider_side === 'toss' || vetoTarget.decider_side === 'pickban'" class="mb-3 p-3 rounded" style="background:rgba(255,255,255,.05)">
								<div class="d-flex align-items-center gap-2 mb-2">
									<span class="fw-semibold">{{ mapDisplayName(vetoRemaining[0]) }}</span>
									<span class="badge bg-warning text-dark ms-1">Décisive</span>
								</div>
								<div class="small text-inverse text-opacity-50 mb-2">Camp de départ de {{ teamName(vetoTarget.team1) }} :</div>
								<div class="d-flex gap-2">
									<button class="btn btn-sm flex-fill"
										:class="vetoDeciderSide === 'ct' ? 'btn-theme' : 'btn-outline-secondary'"
										@click="vetoDeciderSide = 'ct'">
										<i class="fa fa-shield me-1"></i>CT (Défense)
									</button>
									<button class="btn btn-sm flex-fill"
										:class="vetoDeciderSide === 't' ? 'btn-warning' : 'btn-outline-secondary'"
										@click="vetoDeciderSide = 't'">
										<i class="fa fa-bomb me-1"></i>T (Attaque)
									</button>
								</div>
							</div>

							<button class="btn btn-theme w-100" :disabled="!vetoSidesDone()" @click="confirmSides">
								Confirmer les côtés
							</button>
						</template>

						<!-- STEP: done — résumé -->
						<template v-if="vetoStep === 'done'">
							<p class="text-inverse text-opacity-75 mb-3">Récapitulatif du veto :</p>
							<div class="d-flex flex-column gap-2 mb-4">
								<div v-for="(a, i) in vetoActions.filter(x => x.action === 'pick')" :key="a.map"
									class="d-flex align-items-center gap-3 p-2 rounded" style="background:rgba(255,255,255,.05)">
									<span class="badge bg-secondary">Map {{ i + 1 }}</span>
									<span class="fw-semibold">{{ mapDisplayName(a.map) }}</span>
									<span class="text-inverse text-opacity-50 small">Choisi par {{ a.team === 1 ? teamName(vetoTarget.team1) : teamName(vetoTarget.team2) }}</span>
									<span v-if="vetoSides[a.map]?.side" class="ms-auto badge" :class="vetoSides[a.map].side === 'ct' ? 'bg-info' : 'bg-warning text-dark'">
										{{ teamName(vetoTarget.team1) }} {{ vetoSides[a.map].side === 'ct' ? 'CT' : 'T' }}
									</span>
								</div>
								<div class="d-flex align-items-center gap-3 p-2 rounded" style="background:rgba(255,255,255,.05)">
									<span class="badge bg-warning text-dark">Décisive</span>
									<span class="fw-semibold">{{ mapDisplayName(vetoRemaining[0]) }}</span>
									<span v-if="vetoDeciderSide" class="ms-auto badge" :class="vetoDeciderSide === 'ct' ? 'bg-info' : 'bg-warning text-dark'">
										{{ teamName(vetoTarget.team1) }} {{ vetoDeciderSide === 'ct' ? 'CT' : 'T' }}
									</span>
									<span v-else-if="vetoTarget.decider_side === 'knife'" class="ms-auto badge bg-secondary">Couteaux</span>
									<span v-else-if="vetoTarget.decider_side === 'vote'" class="ms-auto badge bg-secondary">Vote joueurs</span>
								</div>
							</div>
							<button class="btn btn-theme w-100" @click="finalizeVeto">
								<i class="fa fa-play me-2"></i>Valider et configurer le serveur
							</button>
						</template>

					</div>
				</div>
			</div>
		</div>

		<div v-if="showStart" class="modal d-block" tabindex="-1" style="background:rgba(0,0,0,.6)">
			<div class="modal-dialog">
				<div class="modal-content">
					<div class="modal-header border-light border-opacity-10">
						<h5 class="modal-title"><i class="fa fa-play me-2 text-theme"></i>Lancer la rencontre</h5>
						<button class="btn-close" @click="showStart = false"></button>
					</div>
					<div class="modal-body">
						<p class="fw-semibold mb-3">
							{{ startTarget?.team1_name || startTarget?.team1 }}
							<span class="text-inverse text-opacity-50 mx-1">vs</span>
							{{ startTarget?.team2_name || startTarget?.team2 }}
						</p>
						<div class="mb-3">
							<label class="form-label">Nom de l'événement <span class="text-inverse text-opacity-50 small">(optionnel)</span></label>
							<input v-model="startForm.label" class="form-control" placeholder="ex: LAN Party 2025, Poule A…" />
							<div class="form-text font-monospace" style="font-size:.78rem">
								Hostname →
								<span class="text-theme">{{ startForm.label ? startForm.label + ' - ' : '' }}{{ startTarget?.team1_name || startTarget?.team1 }} vs {{ startTarget?.team2_name || startTarget?.team2 }} - Warmup</span>
							</div>
						</div>
						<div class="mb-3">
							<label class="form-label">Serveur <span class="text-danger">*</span></label>
							<select v-model="startForm.server_id" class="form-select">
								<option value="" disabled>Sélectionner un serveur…</option>
								<option v-for="s in managedServers" :key="s.id" :value="s.id">
									{{ s.name || s.addr }} — {{ s.map }}
								</option>
							</select>
							<div v-if="managedServers.length === 0" class="form-text text-warning">
								<i class="fa fa-triangle-exclamation me-1"></i>Aucun serveur en ligne avec RCON configuré.
							</div>
						</div>
						<div class="mb-3">
							<label class="form-label">Profil de match <span class="text-danger">*</span></label>
							<select v-model="startForm.profile_id" class="form-select">
								<option value="" disabled>Sélectionner un profil…</option>
								<option v-for="p in compatibleProfiles" :key="p.id" :value="p.id">{{ p.name }}</option>
							</select>
							<div v-if="compatibleProfiles.length === 0" class="form-text text-warning">
								<i class="fa fa-triangle-exclamation me-1"></i>Aucun profil compatible avec ce mode de jeu.
							</div>
						</div>
						<div class="alert alert-info py-2 small mb-0">
							<i class="fa fa-circle-info me-1"></i>
							Le backend va pousser le CFG warmup, renommer le serveur et démarrer l'enregistrement démo.
						</div>
					</div>
					<div class="modal-footer border-light border-opacity-10">
						<button class="btn btn-outline-secondary btn-sm" @click="showStart = false">Annuler</button>
						<button class="btn btn-theme btn-sm" @click="startEncounter"
							:disabled="starting || !startForm.server_id || !startForm.profile_id">
							<span v-if="starting" class="spinner-border spinner-border-sm me-1"></span>
							<i v-else class="fa fa-play me-1"></i>Lancer
						</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Result override modal -->
		<div v-if="showResult" class="modal d-block" tabindex="-1" style="background:rgba(0,0,0,.6)">
			<div class="modal-dialog">
				<div class="modal-content">
					<div class="modal-header border-light border-opacity-10">
						<h5 class="modal-title"><i class="fa fa-pen me-2 text-theme"></i>Saisir le résultat</h5>
						<button class="btn-close" @click="showResult = false"></button>
					</div>
					<div class="modal-body">
						<div class="mb-3" v-if="resultTarget?.maps?.length > 1">
							<label class="form-label">Map</label>
							<select v-model="resultForm.map_number" class="form-select form-select-sm">
								<option v-for="m in resultTarget.maps" :key="m.number" :value="m.number">
									Map {{ m.number }}{{ m.map ? ' — ' + m.map : '' }}
								</option>
							</select>
						</div>
						<div class="row g-3">
							<div class="col-6">
								<label class="form-label">{{ resultTarget?.team1_name || 'Équipe 1' }}</label>
								<input v-model.number="resultForm.score1" type="number" min="0" max="30" class="form-control" />
							</div>
							<div class="col-6">
								<label class="form-label">{{ resultTarget?.team2_name || 'Équipe 2' }}</label>
								<input v-model.number="resultForm.score2" type="number" min="0" max="30" class="form-control" />
							</div>
						</div>
					</div>
					<div class="modal-footer border-light border-opacity-10">
						<button class="btn btn-outline-secondary btn-sm" @click="showResult = false">Annuler</button>
						<button class="btn btn-theme btn-sm" @click="saveResult" :disabled="savingResult">
							<span v-if="savingResult" class="spinner-border spinner-border-sm me-1"></span>
							<i v-else class="fa fa-check me-1"></i>Enregistrer
						</button>
					</div>
				</div>
			</div>
		</div>
	</teleport>
</template>

<style scoped>
.transition-transform { transition: transform .2s ease; }
.bg-inverse.bg-opacity-3 { background-color: rgba(255,255,255,.03) !important; }
@keyframes pulse { 0%,100% { opacity:1 } 50% { opacity:.3 } }
.veto-card {
  width: 110px;
  border-radius: 6px;
  overflow: hidden;
  border: 2px solid rgba(255,255,255,.1);
  transition: border-color .15s, opacity .15s, transform .15s;
  opacity: .5;
}
.veto-card--active { opacity: 1; cursor: pointer; }
.veto-card--active:hover { border-color: rgba(255,255,255,.5); transform: translateY(-2px); }
.veto-card--banned { opacity: .3; }
.veto-card--picked { opacity: 1; border-color: var(--bs-success); }
.veto-card__thumb {
  position: relative;
  aspect-ratio: 16/9;
  background: rgba(255,255,255,.06);
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}
.veto-card__thumb img { width:100%; height:100%; object-fit:cover; display:block; }
.veto-card__overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0,0,0,.5);
}
.veto-card--active .veto-card__overlay { display: none; }
.veto-card__name {
  font-size: .72rem;
  text-align: center;
  padding: 3px 4px;
  background: rgba(0,0,0,.45);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.map-card {
  width: 90px;
  cursor: pointer;
  user-select: none;
  border-radius: 6px;
  overflow: hidden;
  border: 2px solid transparent;
  transition: border-color .15s, opacity .15s;
  opacity: .65;
}
.map-card:hover { opacity: .9; }
.map-card--selected { border-color: var(--bs-theme); opacity: 1; }
.map-card__thumb {
  position: relative;
  aspect-ratio: 16/9;
  background: rgba(255,255,255,.06);
  overflow: hidden;
}
.map-card__thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.map-card__check {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.2rem;
  color: #fff;
  background: rgba(0,0,0,.45);
}
.map-card__name {
  font-size: .72rem;
  text-align: center;
  padding: 3px 4px;
  background: rgba(0,0,0,.35);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.modal-section-label {
  font-size: .65rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: .08em;
  color: rgba(255,255,255,.35);
  border-top: 1px solid rgba(255,255,255,.07);
  margin: 1rem -1rem .75rem;
  padding: .6rem 1rem 0;
}
</style>
