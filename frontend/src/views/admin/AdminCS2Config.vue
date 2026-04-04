<script>
import config from '@/config/matchmaking'
import { mapDisplayName, mapThumbnail } from '@/config/maps'
import { useAuthStore } from '@/stores/auth'

const MODE_PREFIXES = [
	{ prefix: 'de_', label: 'Défuse / Compétitif' },
	{ prefix: 'cs_', label: 'Otages' },
	{ prefix: 'ar_', label: 'Arms Race' },
	{ prefix: 'dm_', label: 'Deathmatch' },
]

export default {
	data() {
		return {
			pool: {},
			loading: true,
			saving: false,
			saved: false,
			newMap: {},  // prefix -> string en cours de saisie
		}
	},
	computed: {
		modePrefixes() { return MODE_PREFIXES },
	},
	mounted() {
		this.fetchPool()
	},
	methods: {
		authHeaders() {
			return { Authorization: `Bearer ${useAuthStore().token}` }
		},
		async fetchPool() {
			this.loading = true
			try {
				const res = await fetch(`${config.api.baseUrl}/map-pool`, { headers: this.authHeaders() })
				this.pool = await res.json()
				// Init newMap for each prefix
				for (const { prefix } of MODE_PREFIXES) {
					this.newMap[prefix] = ''
					if (!this.pool[prefix]) this.pool[prefix] = []
				}
			} finally {
				this.loading = false
			}
		},
		removeMap(prefix, map) {
			const idx = this.pool[prefix]?.indexOf(map)
			if (idx >= 0) this.pool[prefix].splice(idx, 1)
		},
		addMap(prefix) {
			const name = this.newMap[prefix]?.trim().toLowerCase()
			if (!name || !name.startsWith(prefix)) return
			if (!this.pool[prefix]) this.pool[prefix] = []
			if (!this.pool[prefix].includes(name)) this.pool[prefix].push(name)
			this.newMap[prefix] = ''
		},
		async save() {
			this.saving = true
			this.saved = false
			try {
				await fetch(`${config.api.baseUrl}/map-pool`, {
					method: 'PUT',
					headers: { ...this.authHeaders(), 'Content-Type': 'application/json' },
					body: JSON.stringify(this.pool),
				})
				this.saved = true
				setTimeout(() => { this.saved = false }, 2500)
			} finally {
				this.saving = false
			}
		},
		mapDisplayName(map) { return mapDisplayName(map) },
		mapThumbnail(map) { return mapThumbnail(map) },
	},
}
</script>

<template>
	<div>
		<h1 class="page-header">Counter-Strike 2 <small>Configuration</small></h1>

		<div v-if="loading" class="text-center py-5 text-inverse text-opacity-50">
			<i class="fa fa-spinner fa-spin me-2"></i>Chargement…
		</div>

		<template v-else>
			<!-- Map pool officiel -->
			<div class="card mb-4">
				<div class="card-header d-flex align-items-center justify-content-between">
					<span><i class="fa fa-map me-2"></i>Map pool officiel</span>
					<button class="btn btn-sm btn-theme" :disabled="saving" @click="save">
						<i class="fa fa-save me-1"></i>
						<span v-if="saving">Enregistrement…</span>
						<span v-else-if="saved"><i class="fa fa-check me-1"></i>Enregistré</span>
						<span v-else>Enregistrer</span>
					</button>
				</div>
				<div class="card-body">
					<p class="text-inverse text-opacity-50 small mb-3">
						Cartes proposées par défaut dans le pick &amp; ban lors de la création d'une rencontre.
						Mis à jour manuellement lors des changements de pool Valve.
					</p>

					<div v-for="{ prefix, label } in modePrefixes" :key="prefix" class="mb-4">
						<div class="fw-semibold small text-inverse text-opacity-75 mb-2">
							{{ label }}
							<span class="badge bg-secondary ms-2">{{ (pool[prefix] || []).length }}</span>
						</div>

						<!-- Cards -->
						<div class="d-flex flex-wrap gap-2 mb-2">
							<div v-for="map in (pool[prefix] || [])" :key="map" class="map-card">
								<div class="map-card__thumb">
									<img :src="mapThumbnail(map)" :alt="mapDisplayName(map)"
										@error="$event.target.style.display='none'" />
									<button class="map-card__remove" type="button" @click="removeMap(prefix, map)"
										title="Retirer">
										<i class="fa fa-xmark"></i>
									</button>
								</div>
								<div class="map-card__name">{{ mapDisplayName(map) }}</div>
							</div>

							<!-- Add card -->
							<div class="map-card map-card--add">
								<div class="map-card__thumb map-card__thumb--add">
									<i class="fa fa-plus text-inverse text-opacity-25"></i>
								</div>
								<div class="map-card__add-input">
									<input
										v-model="newMap[prefix]"
										type="text"
										class="form-control form-control-sm"
										:placeholder="`${prefix}…`"
										@keydown.enter="addMap(prefix)"
										@keydown.space.prevent="addMap(prefix)" />
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</template>
	</div>
</template>

<style scoped>
.map-card {
	width: 90px;
	border-radius: 6px;
	overflow: hidden;
	border: 2px solid rgba(255,255,255,.1);
}
.map-card__thumb {
	position: relative;
	aspect-ratio: 16/9;
	background: rgba(255,255,255,.06);
	overflow: hidden;
	display: flex;
	align-items: center;
	justify-content: center;
}
.map-card__thumb img {
	width: 100%;
	height: 100%;
	object-fit: cover;
	display: block;
}
.map-card__remove {
	position: absolute;
	top: 3px;
	right: 3px;
	width: 18px;
	height: 18px;
	border-radius: 50%;
	border: none;
	background: rgba(0,0,0,.7);
	color: #fff;
	font-size: .6rem;
	cursor: pointer;
	display: flex;
	align-items: center;
	justify-content: center;
	opacity: 0;
	transition: opacity .15s;
}
.map-card:hover .map-card__remove { opacity: 1; }
.map-card__name {
	font-size: .72rem;
	text-align: center;
	padding: 3px 4px;
	background: rgba(0,0,0,.35);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}
.map-card--add { border-style: dashed; opacity: .6; }
.map-card--add:focus-within { opacity: 1; }
.map-card__thumb--add { cursor: default; }
.map-card__add-input { padding: 3px 4px; background: rgba(0,0,0,.35); }
.map-card__add-input .form-control { font-size: .7rem; padding: 1px 4px; height: auto; }
</style>
