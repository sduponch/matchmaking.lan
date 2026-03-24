<script>
import config from '@/config/matchmaking'
import { useAuthStore } from '@/stores/auth'

export default {
	data() {
		return {
			servers: [],
			loading: false,
			selected: [],
			bulkRcon: '',
			bulkAdding: false,
			bulkResults: [],
			showAddModal: false,
			newAddr: '',
			newRcon: '',
			adding: false,
			addError: '',
		}
	},
	mounted() {
		this.fetchServers()
	},
	computed: {
		unmanagedServers() {
			return (this.servers || []).filter(s => !s.managed)
		},
		managedServers() {
			return (this.servers || []).filter(s => s.managed)
		},
		allSelected() {
			return this.unmanagedServers.length > 0 && this.selected.length === this.unmanagedServers.length
		},
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
				this.selected = this.selected.filter(addr => this.unmanagedServers.some(s => s.addr === addr))
			} catch {
				this.servers = []
			} finally {
				this.loading = false
			}
		},
		toggleAll() {
			if (this.allSelected) {
				this.selected = []
			} else {
				this.selected = this.unmanagedServers.map(s => s.addr)
			}
		},
		toggleSelect(addr) {
			const idx = this.selected.indexOf(addr)
			if (idx === -1) this.selected.push(addr)
			else this.selected.splice(idx, 1)
		},
		openAddModal() {
			this.newAddr = ''
			this.newRcon = ''
			this.addError = ''
			this.showAddModal = true
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
					this.showAddModal = false
					await this.fetchServers()
				} else {
					const data = await res.json()
					this.addError = data.error || 'Erreur inconnue'
				}
			} finally {
				this.adding = false
			}
		},
		async configureBulk() {
			if (!this.selected.length || !this.bulkRcon.trim()) return
			this.bulkAdding = true
			this.bulkResults = []
			const results = await Promise.all(
				this.selected.map(addr =>
					fetch(`${config.api.baseUrl}/servers`, {
						method: 'POST',
						headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
						body: JSON.stringify({ addr, rcon: this.bulkRcon.trim() }),
					})
					.then(async r => ({ addr, ok: r.ok, error: r.ok ? null : (await r.json()).error }))
					.catch(() => ({ addr, ok: false, error: 'Erreur réseau' }))
				)
			)
			this.bulkResults = results
			this.bulkRcon = ''
			this.bulkAdding = false
			await this.fetchServers()
		},
	},
}
</script>

<template>
	<ul class="breadcrumb">
		<li class="breadcrumb-item">Administration</li>
		<li class="breadcrumb-item">Serveurs</li>
		<li class="breadcrumb-item active">Configurer</li>
	</ul>

	<!-- Table des serveurs -->
	<card>
		<card-header class="d-flex align-items-center justify-content-between fw-semibold">
			<div class="d-flex align-items-center gap-2">
				<i class="fa fa-wifi me-1 text-theme"></i>Serveurs détectés sur le LAN
				<span v-if="!loading" class="badge bg-inverse bg-opacity-15 text-inverse fw-normal">{{ unmanagedServers.length }}</span>
			</div>
			<div class="d-flex align-items-center gap-2">
				<span v-if="!loading" class="badge bg-theme text-theme-900">
					{{ managedServers.length }} configuré{{ managedServers.length > 1 ? 's' : '' }}
				</span>
				<button class="btn btn-outline-theme btn-sm" @click="openAddModal" title="Ajouter manuellement">
					<i class="fa fa-plus"></i>
				</button>
				<button class="btn btn-outline-theme btn-sm" @click="fetchServers" :disabled="loading" title="Actualiser">
					<i class="fa fa-rotate-right" :class="{ 'fa-spin': loading }"></i>
				</button>
			</div>
		</card-header>

		<div v-if="loading" class="py-5 text-center">
			<div class="spinner-border text-theme" role="status"></div>
		</div>

		<div v-else-if="unmanagedServers.length === 0" class="py-5 d-flex flex-column align-items-center">
			<i class="fa fa-server fa-2x text-inverse text-opacity-15 mb-3"></i>
			<p class="text-inverse text-opacity-50 small mb-1">Aucun serveur non configuré détecté sur le LAN.</p>
			<p class="text-inverse text-opacity-25 small mb-0">Utilisez <strong>Ajouter manuellement</strong> pour en ajouter un.</p>
		</div>

		<div v-else class="table-responsive">
			<table class="table table-hover table-striped mb-0">
				<thead>
					<tr>
						<th scope="col" style="width:36px">
							<input
								type="checkbox"
								class="form-check-input"
								:checked="allSelected"
								:indeterminate.prop="selected.length > 0 && !allSelected"
								@change="toggleAll"
							/>
						</th>
						<th scope="col">Nom</th>
						<th scope="col">Adresse</th>
						<th scope="col">Statut</th>
						<th scope="col">Map</th>
						<th scope="col">Joueurs</th>
						<th scope="col">Ping</th>
					</tr>
				</thead>
				<tbody>
					<tr
						v-for="srv in unmanagedServers"
						:key="srv.addr"
						class="align-middle"
						:class="{ 'table-active': selected.includes(srv.addr) }"
						style="cursor:pointer"
						@click="toggleSelect(srv.addr)"
					>
						<td class="px-3">
							<input
								type="checkbox"
								class="form-check-input"
								:checked="selected.includes(srv.addr)"
								@click.stop
								@change="toggleSelect(srv.addr)"
							/>
						</td>
						<td class="fw-semibold">{{ srv.online ? srv.name : '—' }}</td>
						<td class="font-monospace text-inverse text-opacity-50" style="font-size:.82rem">{{ srv.addr }}</td>
						<td>
							<span class="badge d-inline-flex align-items-center px-2 pt-5px pb-5px rounded fs-12px"
								:class="srv.online ? 'border border-success text-success' : 'border border-danger text-danger'">
								<i class="fa fa-circle fs-9px fa-fw me-5px"></i>{{ srv.online ? 'En ligne' : 'Hors ligne' }}
							</span>
						</td>
						<td>{{ srv.online ? srv.map : '—' }}</td>
						<td>{{ srv.online ? `${srv.players} / ${srv.max_players}` : '—' }}</td>
						<td>{{ srv.online ? `${srv.ping_ms} ms` : '—' }}</td>
					</tr>
				</tbody>
			</table>
		</div>

		<!-- RCON commun (visible si sélection) -->
		<transition name="fade">
			<div v-if="selected.length > 0" class="card-footer border-top border-light border-opacity-10">
				<div class="d-flex align-items-center gap-3 flex-wrap">
					<span class="text-theme fw-semibold small">
						<i class="fa fa-circle-check me-1"></i>{{ selected.length }} serveur{{ selected.length > 1 ? 's' : '' }} sélectionné{{ selected.length > 1 ? 's' : '' }}
					</span>
					<div class="input-group input-group-sm" style="max-width: 260px">
						<span class="input-group-text"><i class="fa fa-key fa-xs"></i></span>
						<input
							v-model="bulkRcon"
							type="password"
							class="form-control"
							placeholder="RCON commun…"
							@keyup.enter="configureBulk"
						/>
					</div>
					<button class="btn btn-theme btn-sm" :disabled="bulkAdding || !bulkRcon.trim()" @click="configureBulk">
						<span v-if="bulkAdding" class="spinner-border spinner-border-sm me-1"></span>
						<i v-else class="fa fa-link me-1"></i>Configurer
					</button>
					<button class="btn btn-outline-secondary btn-sm ms-auto" @click="selected = []">
						<i class="fa fa-xmark me-1"></i>Désélectionner
					</button>
				</div>
				<div v-if="bulkResults.length" class="mt-2 d-flex flex-wrap gap-2">
					<span
						v-for="r in bulkResults" :key="r.addr"
						class="badge"
						:class="r.ok ? 'bg-success bg-opacity-20 text-success' : 'bg-danger bg-opacity-20 text-danger'"
					>
						<i :class="r.ok ? 'fa fa-check' : 'fa fa-xmark'" class="me-1"></i>
						{{ r.addr }}<span v-if="!r.ok"> — {{ r.error }}</span>
					</span>
				</div>
			</div>
		</transition>
	</card>

	<!-- Modal ajout manuel -->
	<teleport to="body">
		<transition name="fade">
			<div v-if="showAddModal" class="modal-backdrop show" style="z-index:1040" @click="showAddModal = false"></div>
		</transition>
		<transition name="slide-up">
			<div v-if="showAddModal" class="modal d-block" style="z-index:1050" tabindex="-1" @click.self="showAddModal = false">
				<div class="modal-dialog modal-dialog-centered">
					<div class="modal-content">
						<div class="modal-header border-0 pb-0">
							<h5 class="modal-title fw-semibold">
								<i class="fa fa-plus me-2 text-theme"></i>Ajouter un serveur
							</h5>
							<button type="button" class="btn-close btn-close-white opacity-25" @click="showAddModal = false"></button>
						</div>
						<div class="modal-body pt-3">
							<div class="mb-3">
								<label class="form-label small fw-semibold">Adresse IP</label>
								<input v-model="newAddr" type="text" class="form-control" placeholder="192.168.1.10:27015" @keyup.enter="addServer" />
							</div>
							<div class="mb-3">
								<label class="form-label small fw-semibold">Mot de passe RCON</label>
								<input v-model="newRcon" type="password" class="form-control" placeholder="••••••••" @keyup.enter="addServer" />
							</div>
							<div v-if="addError" class="alert alert-danger py-2 small">
								<i class="fa fa-triangle-exclamation me-2"></i>{{ addError }}
							</div>
						</div>
						<div class="modal-footer border-0 pt-0">
							<button type="button" class="btn btn-outline-secondary" @click="showAddModal = false">Annuler</button>
							<button type="button" class="btn btn-theme" @click="addServer" :disabled="adding || !newAddr.trim()">
								<span v-if="adding" class="spinner-border spinner-border-sm me-2"></span>
								<i v-else class="fa fa-link me-2"></i>Ajouter et configurer
							</button>
						</div>
					</div>
				</div>
			</div>
		</transition>
	</teleport>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity .2s }
.fade-enter-from, .fade-leave-to { opacity: 0 }
.slide-up-enter-active, .slide-up-leave-active { transition: opacity .2s, transform .2s }
.slide-up-enter-from, .slide-up-leave-to { opacity: 0; transform: translateY(16px) }
</style>
