<script>
import config from '@/config/matchmaking'
import { useAuthStore } from '@/stores/auth'

const PHASES = ['warmup', 'knife', 'live', 'halftime', 'game_over']
const PHASE_LABELS = {
	warmup:    'Warmup',
	knife:     'Couteaux',
	live:      'Live',
	halftime:  'Mi-temps',
	game_over: 'Fin de match',
}

const GAME_MODES = [
	{ value: 'defuse',     label: 'Défuse',          icon: 'fa-bomb' },
	{ value: 'casual',     label: 'Occasionnel',     icon: 'fa-people-group' },
	{ value: 'wingman',    label: 'Wingman',         icon: 'fa-user-group' },
	{ value: 'retakes',    label: 'Reprise contrôle', icon: 'fa-rotate-left' },
	{ value: 'hostage',    label: 'Otages',          icon: 'fa-person-walking-arrow-right' },
	{ value: 'armsrace',   label: 'Arms Race',       icon: 'fa-gun' },
	{ value: 'deathmatch', label: 'Deathmatch',      icon: 'fa-skull' },
]

const GAME_MODES_MAP = Object.fromEntries(GAME_MODES.map(m => [m.value, m]))

export default {
	data() {
		return {
			profiles: [],
			loading: true,
			openProfile: null,
			activeTab: {},        // profileId → phase
			cfgDraft: {},         // profileId+phase → content string
			cfgSaving: {},        // profileId+phase → bool
			serverInitContent: '',
			serverInitSaving: false,
			serverInitLoaded: false,
			// New profile modal
			showModal: false,
			newProfile: {
				name: '',
				tags: [],
			},
			creating: false,
			// Edit profile metadata
			editingMeta: null,   // profileId
			metaDraft: {},
		}
	},
	mounted() {
		this.fetchProfiles()
		this.fetchServerInit()
	},
	methods: {
		authHeaders() {
			return { Authorization: `Bearer ${useAuthStore().token}` }
		},
		async fetchProfiles() {
			this.loading = true
			try {
				const res = await fetch(`${config.api.baseUrl}/match-profiles`, { headers: this.authHeaders() })
				this.profiles = await res.json()
				// Initialise active tab per profile
				for (const p of this.profiles) {
					if (!this.activeTab[p.id]) this.activeTab[p.id] = 'warmup'
				}
			} catch {
				this.profiles = []
			} finally {
				this.loading = false
			}
		},
		async fetchServerInit() {
			try {
				const res = await fetch(`${config.api.baseUrl}/server-init-cfg`, { headers: this.authHeaders() })
				const data = await res.json()
				this.serverInitContent = data.content || ''
				this.serverInitLoaded = true
			} catch {}
		},
		toggle(id) {
			if (this.openProfile === id) {
				this.openProfile = null
				return
			}
			this.openProfile = id
			const p = this.profiles.find(x => x.id === id)
			if (p) this.loadCFGsForProfile(p)
		},
		loadCFGsForProfile(p) {
			for (const phase of PHASES) {
				const key = p.id + ':' + phase
				if (!(key in this.cfgDraft)) {
					this.cfgDraft[key] = (p.cfgs && p.cfgs[phase]) || ''
				}
			}
		},
		cfgKey(profileId, phase) {
			return profileId + ':' + phase
		},
		async saveCFG(profileId, phase) {
			const key = this.cfgKey(profileId, phase)
			this.cfgSaving[key] = true
			try {
				await fetch(`${config.api.baseUrl}/match-profiles/${profileId}/cfg/${phase}`, {
					method: 'PUT',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ content: this.cfgDraft[key] || '' }),
				})
			} finally {
				this.cfgSaving[key] = false
			}
		},
		async saveServerInit() {
			this.serverInitSaving = true
			try {
				await fetch(`${config.api.baseUrl}/server-init-cfg`, {
					method: 'PUT',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify({ content: this.serverInitContent }),
				})
			} finally {
				this.serverInitSaving = false
			}
		},
		openCreateModal() {
			this.newProfile = { name: '', tags: [] }
			this.showModal = true
		},
		async createProfile() {
			if (!this.newProfile.name.trim()) return
			this.creating = true
			try {
				const res = await fetch(`${config.api.baseUrl}/match-profiles`, {
					method: 'POST',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify(this.newProfile),
				})
				const created = await res.json()
				this.profiles.push(created)
				this.activeTab[created.id] = 'warmup'
				this.showModal = false
			} finally {
				this.creating = false
			}
		},
		gameModes() { return GAME_MODES },
		gameModeInfo(val) { return GAME_MODES_MAP[val] },
		toggleTag(tags, val) {
			const idx = tags.indexOf(val)
			if (idx >= 0) tags.splice(idx, 1)
			else tags.push(val)
		},
		startEditMeta(p) {
			this.editingMeta = p.id
			this.metaDraft = {
				name: p.name,
				tags: [...(p.tags || [])],
			}
		},
		cancelEditMeta() {
			this.editingMeta = null
		},
		async saveEditMeta(p) {
			await fetch(`${config.api.baseUrl}/match-profiles/${p.id}`, {
				method: 'PUT',
				headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
				body: JSON.stringify(this.metaDraft),
			})
			Object.assign(p, this.metaDraft)
			this.editingMeta = null
		},
		async deleteProfile(p) {
			if (!confirm(`Supprimer le profil "${p.name}" ?`)) return
			await fetch(`${config.api.baseUrl}/match-profiles/${p.id}`, {
				method: 'DELETE',
				headers: this.authHeaders(),
			})
			if (this.openProfile === p.id) this.openProfile = null
			this.profiles = this.profiles.filter(x => x.id !== p.id)
		},
		phaseLabel(phase) {
			return PHASE_LABELS[phase] || phase
		},
		formatLabel(f) {
			return { bo1: 'BO1', bo3: 'BO3', bo5: 'BO5' }[f] || f
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item active">Profils de match</li>
	</ul>

	<!-- Server init CFG -->
	<card class="mb-4">
		<card-header class="fw-semibold d-flex align-items-center gap-2">
			<i class="fa fa-file-code text-theme me-1"></i>Config init serveur
			<span class="text-inverse text-opacity-50 fw-normal small">(server_init.cfg — poussée à chaque ajout de serveur)</span>
		</card-header>
		<card-body>
			<div v-if="!serverInitLoaded" class="text-center py-3">
				<div class="spinner-border spinner-border-sm text-theme"></div>
			</div>
			<template v-else>
				<textarea
					v-model="serverInitContent"
					class="form-control font-monospace"
					rows="10"
					style="font-size:.82rem;resize:vertical"
					placeholder="// Commandes poussées lors de l'ajout du serveur&#10;// ex: gotv_enable 1"
				></textarea>
				<div class="d-flex justify-content-end mt-2">
					<button class="btn btn-outline-theme btn-sm" @click="saveServerInit" :disabled="serverInitSaving">
						<span v-if="serverInitSaving" class="spinner-border spinner-border-sm me-1"></span>
						<i v-else class="fa fa-floppy-disk me-1"></i>Enregistrer
					</button>
				</div>
			</template>
		</card-body>
	</card>

	<!-- Match profiles -->
	<card>
		<card-header class="d-flex align-items-center justify-content-between fw-semibold">
			<div class="d-flex align-items-center gap-2">
				<i class="fa fa-sliders text-theme me-1"></i>Profils de match
				<span class="badge bg-inverse bg-opacity-15 text-inverse fw-normal">{{ profiles.length }}</span>
			</div>
			<div class="d-flex gap-2">
				<button class="btn btn-outline-theme btn-sm" @click="fetchProfiles" :disabled="loading">
					<i class="fa fa-rotate-right" :class="{ 'fa-spin': loading }"></i>
				</button>
				<button class="btn btn-theme btn-sm" @click="openCreateModal">
					<i class="fa fa-plus me-1"></i>Nouveau profil
				</button>
			</div>
		</card-header>

		<div v-if="loading" class="text-center py-5">
			<div class="spinner-border text-theme" role="status"></div>
		</div>

		<div v-else-if="profiles.length === 0" class="card-body">
			<p class="text-inverse text-opacity-50 mb-0">Aucun profil. Créez-en un pour commencer.</p>
		</div>

		<div v-else>
			<template v-for="p in profiles" :key="p.id">
				<!-- Header row -->
				<div
					class="d-flex align-items-center px-4 py-3 border-bottom border-light border-opacity-10"
					style="cursor:pointer"
					:class="{ 'bg-inverse bg-opacity-5': openProfile === p.id }"
					@click="toggle(p.id)"
				>
					<i class="fa fa-chevron-right fa-xs text-inverse text-opacity-25 me-3 transition-transform flex-shrink-0"
						:style="openProfile === p.id ? 'transform:rotate(90deg)' : ''">
					</i>
					<div class="flex-grow-1">
						<span class="fw-semibold me-2">{{ p.name }}</span>
						<template v-if="p.tags?.length">
							<span v-for="tag in p.tags" :key="tag" class="badge bg-theme bg-opacity-15 text-theme fw-normal me-1">
								<i :class="['fa', gameModeInfo(tag)?.icon ?? 'fa-tag', 'me-1']"></i>{{ gameModeInfo(tag)?.label ?? tag }}
							</span>
						</template>
						<span v-else class="badge bg-secondary bg-opacity-15 text-secondary fw-normal">
							<i class="fa fa-globe me-1"></i>Tous modes
						</span>
					</div>
					<div class="d-flex gap-2" @click.stop>
						<button class="btn btn-outline-secondary btn-sm" @click="startEditMeta(p)" title="Modifier">
							<i class="fa fa-pen"></i>
						</button>
						<button class="btn btn-outline-danger btn-sm" @click="deleteProfile(p)" title="Supprimer">
							<i class="fa fa-trash"></i>
						</button>
					</div>
				</div>

				<!-- Expanded content -->
				<div v-if="openProfile === p.id" class="px-4 py-3 border-bottom border-light border-opacity-10 bg-inverse bg-opacity-3">

					<!-- Edit metadata form -->
					<div v-if="editingMeta === p.id" class="mb-4">
						<p class="text-inverse text-opacity-50 text-uppercase fw-semibold mb-3" style="font-size:.65rem;letter-spacing:.1em">
							<i class="fa fa-pen me-1"></i>Modifier le profil
						</p>
						<div class="row g-3">
							<div class="col-sm-6">
								<label class="form-label form-label-sm">Nom</label>
								<input v-model="metaDraft.name" class="form-control form-control-sm" />
							</div>
						</div>
						<div class="mt-3">
							<label class="form-label form-label-sm">Modes de jeu compatibles <span class="text-inverse text-opacity-50">(vide = tous)</span></label>
							<div class="d-flex flex-wrap gap-2">
								<button v-for="m in gameModes" :key="m.value" type="button"
									class="btn btn-sm"
									:class="metaDraft.tags.includes(m.value) ? 'btn-theme' : 'btn-outline-secondary'"
									@click="toggleTag(metaDraft.tags, m.value)">
									<i :class="['fa', m.icon, 'me-1']"></i>{{ m.label }}
								</button>
							</div>
						</div>
						<div class="d-flex gap-2 mt-3">
							<button class="btn btn-outline-theme btn-sm" @click="saveEditMeta(p)">
								<i class="fa fa-check me-1"></i>Sauvegarder
							</button>
							<button class="btn btn-outline-secondary btn-sm" @click="cancelEditMeta">Annuler</button>
						</div>
						<hr class="border-light border-opacity-10 mt-4" />
					</div>

					<!-- Phase CFG tabs -->
					<p class="text-inverse text-opacity-50 text-uppercase fw-semibold mb-3" style="font-size:.65rem;letter-spacing:.1em">
						<i class="fa fa-file-code me-1"></i>CFG par phase
					</p>
					<ul class="nav nav-tabs mb-3">
						<li v-for="phase in ['warmup','knife','live','halftime','game_over']" :key="phase" class="nav-item">
							<a
								class="nav-link py-1 px-3"
								:class="{ active: (activeTab[p.id] || 'warmup') === phase }"
								style="cursor:pointer;font-size:.85rem"
								@click="activeTab[p.id] = phase"
							>
								{{ phaseLabel(phase) }}
							</a>
						</li>
					</ul>

					<template v-for="phase in ['warmup','knife','live','halftime','game_over']" :key="phase">
						<div v-if="(activeTab[p.id] || 'warmup') === phase">
							<textarea
								v-model="cfgDraft[cfgKey(p.id, phase)]"
								class="form-control font-monospace"
								rows="14"
								style="font-size:.82rem;resize:vertical"
								:placeholder="`// Commandes exécutées en phase ${phaseLabel(phase)}`"
							></textarea>
							<div class="d-flex justify-content-end mt-2">
								<button
									class="btn btn-outline-theme btn-sm"
									@click="saveCFG(p.id, phase)"
									:disabled="cfgSaving[cfgKey(p.id, phase)]"
								>
									<span v-if="cfgSaving[cfgKey(p.id, phase)]" class="spinner-border spinner-border-sm me-1"></span>
									<i v-else class="fa fa-floppy-disk me-1"></i>Enregistrer {{ phaseLabel(phase) }}
								</button>
							</div>
						</div>
					</template>
				</div>
			</template>
		</div>
	</card>

	<!-- Create profile modal -->
	<teleport to="body">
		<div v-if="showModal" class="modal d-block" tabindex="-1" style="background:rgba(0,0,0,.6)">
			<div class="modal-dialog">
				<div class="modal-content">
					<div class="modal-header border-light border-opacity-10">
						<h5 class="modal-title"><i class="fa fa-plus me-2 text-theme"></i>Nouveau profil de match</h5>
						<button type="button" class="btn-close" @click="showModal = false"></button>
					</div>
					<div class="modal-body">
						<div class="mb-3">
							<label class="form-label">Nom <span class="text-danger">*</span></label>
							<input v-model="newProfile.name" class="form-control" placeholder="ex: 5v5 Compétitif" />
						</div>
						<div class="mb-3">
							<label class="form-label">Modes de jeu compatibles <span class="text-inverse text-opacity-50 small">(vide = tous)</span></label>
							<div class="d-flex flex-wrap gap-2">
								<button v-for="m in gameModes" :key="m.value" type="button"
									class="btn btn-sm"
									:class="newProfile.tags.includes(m.value) ? 'btn-theme' : 'btn-outline-secondary'"
									@click="toggleTag(newProfile.tags, m.value)">
									<i :class="['fa', m.icon, 'me-1']"></i>{{ m.label }}
								</button>
							</div>
						</div>
					</div>
					<div class="modal-footer border-light border-opacity-10">
						<button class="btn btn-outline-secondary btn-sm" @click="showModal = false">Annuler</button>
						<button class="btn btn-theme btn-sm" @click="createProfile" :disabled="creating || !newProfile.name.trim()">
							<span v-if="creating" class="spinner-border spinner-border-sm me-1"></span>
							<i v-else class="fa fa-plus me-1"></i>Créer
						</button>
					</div>
				</div>
			</div>
		</div>
	</teleport>
</template>

<style scoped>
.transition-transform {
	transition: transform .2s ease;
}
.bg-inverse.bg-opacity-3 {
	background-color: rgba(255,255,255,.03) !important;
}
</style>
