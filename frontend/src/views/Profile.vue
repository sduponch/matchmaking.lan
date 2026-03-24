<script>
import { useAuthStore } from '@/stores/auth';
import config from '@/config/matchmaking';

export default {
	data() {
		return {
			profile: null,
			loading: true,
			error: null,
			statsLoading: false,
			faceitLoading: false,
			_cs2Timer: null,
			_faceitTimer: null,
		}
	},
	computed: {
		statusClass() {
			const map = { online: 'text-success', busy: 'text-danger', away: 'text-warning', snooze: 'text-warning' };
			return map[this.profile?.status] ?? 'text-secondary';
		},
		statusLabel() {
			const map = { online: 'En ligne', busy: 'Occupé', away: 'Absent', snooze: 'Indisponible', offline: 'Hors ligne' };
			return map[this.profile?.status] ?? 'Hors ligne';
		},
		createdYear() {
			if (!this.profile?.created_at) return '—';
			return new Date(this.profile.created_at).getFullYear();
		}
	},
	async mounted() {
		const auth = useAuthStore();
		try {
			const profileRes = await fetch(`${config.api.baseUrl}/profile/${auth.user.steamid}`, {
				headers: { Authorization: `Bearer ${auth.token}` }
			});

			if (!profileRes.ok) throw new Error('Erreur serveur');
			this.profile = await profileRes.json();
			this.profile.role = auth.user.role;

			if (this.profile.cs2_status === 'retrieving') {
				this.statsLoading = true;
				this._startCS2Poll(auth);
			}
			if (this.profile.faceit_status === 'retrieving') {
				this.faceitLoading = true;
				this._startFaceitPoll(auth);
			}
		} catch (e) {
			this.error = e.message;
		} finally {
			this.loading = false;
		}
	},
	beforeUnmount() {
		this._stopCS2Poll();
		this._stopFaceitPoll();
	},
	methods: {
		_startCS2Poll(auth) {
			this._cs2Timer = setInterval(async () => {
				try {
					const res = await fetch(`${config.api.baseUrl}/profile/${auth.user.steamid}/cs2`, {
						headers: { Authorization: `Bearer ${auth.token}` }
					});
					if (!res.ok) return;
					const data = await res.json();
					if (data.status !== 'retrieving') {
						this.profile.cs2_status       = data.status;
						this.profile.premier_rating   = data.premier_rating;
						this.profile.competitive_rank = data.competitive_rank;
						this.profile.competitive_wins = data.competitive_wins;
						this.statsLoading = false;
						this._stopCS2Poll();
					}
				} catch { /* retry */ }
			}, 2000);
		},
		_stopCS2Poll() {
			if (this._cs2Timer) { clearInterval(this._cs2Timer); this._cs2Timer = null; }
		},
		_startFaceitPoll(auth) {
			this._faceitTimer = setInterval(async () => {
				try {
					const res = await fetch(`${config.api.baseUrl}/profile/${auth.user.steamid}/faceit`, {
						headers: { Authorization: `Bearer ${auth.token}` }
					});
					if (!res.ok) return;
					const data = await res.json();
					if (data.status !== 'retrieving') {
						this.profile.faceit_status    = data.status;
						this.profile.faceit_elo       = data.faceit_elo;
						this.profile.faceit_level     = data.faceit_level;
						this.profile.faceit_nickname  = data.faceit_nickname;
						this.profile.faceit_url       = data.faceit_url;
						this.profile.faceit_matches   = data.faceit_matches;
						this.profile.faceit_win_rate  = data.faceit_win_rate;
						this.profile.faceit_kd_ratio  = data.faceit_kd_ratio;
						this.profile.faceit_headshots = data.faceit_headshots;
						this.faceitLoading = false;
						this._stopFaceitPoll();
					}
				} catch { /* retry */ }
			}, 2000);
		},
		_stopFaceitPoll() {
			if (this._faceitTimer) { clearInterval(this._faceitTimer); this._faceitTimer = null; }
		},
	}
}
</script>

<template>
	<div>
		<!-- Loading -->
		<div v-if="loading" class="d-flex justify-content-center align-items-center" style="min-height: 300px">
			<div class="spinner-border text-theme" role="status"></div>
		</div>

		<!-- Error -->
		<div v-else-if="error" class="alert alert-danger">{{ error }}</div>

		<!-- Profile -->
		<card v-else>
			<card-body class="p-0">
				<div class="profile">
					<div class="profile-container">

						<!-- Sidebar -->
						<div class="profile-sidebar">
							<div class="desktop-sticky-top">
								<div class="profile-img">
									<img :src="profile.avatar" :alt="profile.username" />
								</div>
								<h4>{{ profile.username }}</h4>
								<div v-if="profile.real_name" class="mb-1 text-inverse text-opacity-50 fw-bold mt-n2">
									{{ profile.real_name }}
								</div>

								<div class="mb-2 mt-2">
									<span :class="statusClass">●</span>
									<span class="ms-1 text-inverse text-opacity-75">{{ statusLabel }}</span>
								</div>

								<div v-if="profile.country" class="mb-1">
									<i class="fa fa-map-marker-alt fa-fw text-inverse text-opacity-50"></i>
									{{ profile.country }}
								</div>
								<div class="mb-1">
									<i class="fa fa-link fa-fw text-inverse text-opacity-50"></i>
									<a :href="profile.profile_url" target="_blank" class="text-decoration-none ms-1">
										Profil Steam
									</a>
								</div>
								<div class="mb-3">
									<i class="fa fa-calendar fa-fw text-inverse text-opacity-50"></i>
									<span class="ms-1 text-inverse text-opacity-75">Membre depuis {{ createdYear }}</span>
								</div>

								<hr class="mt-3 mb-3" />

								<div class="row text-center">
									<div class="col-6">
										<div class="fs-24px fw-bold text-theme">{{ profile.steam_level }}</div>
										<div class="fs-12px text-inverse text-opacity-50">Niveau Steam</div>
									</div>
									<div class="col-6">
										<div class="fs-24px fw-bold text-theme text-capitalize">{{ profile.role ?? '—' }}</div>
										<div class="fs-12px text-inverse text-opacity-50">Rôle</div>
									</div>
								</div>

								<hr class="mt-3 mb-3" />

								<!-- Stats CS2 -->
								<div v-if="profile.cs2_status === 'retrieving'" class="text-center py-2">
									<div class="spinner-border spinner-border-sm text-theme" role="status"></div>
									<div class="fs-12px text-inverse text-opacity-50 mt-1">Récupération des stats…</div>
								</div>
								<div v-else-if="profile.cs2_status === 'unavailable'" class="text-center py-2">
									<div class="fs-12px text-inverse text-opacity-50">Stats CS2 indisponibles</div>
								</div>
								<div v-else class="row text-center">
									<div class="col-12 mb-3">
										<div class="fs-28px fw-bold" :class="profile.premier_rating > 0 ? 'text-theme' : 'text-inverse text-opacity-25'">
											{{ profile.premier_rating > 0 ? profile.premier_rating.toLocaleString() : '—' }}
										</div>
										<div class="fs-12px text-inverse text-opacity-50">Rating Premier CS2</div>
									</div>
									<div class="col-6">
										<div class="fs-20px fw-bold" :class="profile.competitive_rank > 0 ? 'text-theme' : 'text-inverse text-opacity-25'">
											{{ profile.competitive_rank > 0 ? profile.competitive_rank : '—' }}
										</div>
										<div class="fs-12px text-inverse text-opacity-50">Rang Compétitif</div>
									</div>
									<div class="col-6">
										<div class="fs-20px fw-bold text-theme">{{ profile.competitive_wins ?? 0 }}</div>
										<div class="fs-12px text-inverse text-opacity-50">Victoires</div>
									</div>
								</div>

								<template v-if="profile.cs2_status === 'pending_invite'">
									<hr class="mt-3 mb-3" />
									<div class="text-center text-inverse text-opacity-50" style="font-size: .75rem">
										Rang Premier en attente — une demande d'ami vous a été envoyée par le bot.
									</div>
								</template>
							</div>
						</div>
						<!-- END sidebar -->

						<!-- Content -->
						<div class="profile-content">
							<ul class="profile-tab nav nav-tabs nav-tabs-v2">
								<li class="nav-item">
									<a href="#tab-faceit" class="nav-link active" data-bs-toggle="tab">
										<div class="nav-field">Faceit</div>
									</a>
								</li>
								<li class="nav-item">
									<a href="#tab-matchmaking" class="nav-link" data-bs-toggle="tab">
										<div class="nav-field">Matchmaking</div>
									</a>
								</li>
							</ul>

							<div class="profile-content-container tab-content p-4">

								<!-- Faceit -->
								<div class="tab-pane fade show active" id="tab-faceit">
									<div v-if="profile.faceit_status === 'retrieving'" class="d-flex align-items-center gap-2 text-inverse text-opacity-50">
										<div class="spinner-border spinner-border-sm text-theme" role="status"></div>
										Chargement des stats Faceit…
									</div>
									<div v-else-if="profile.faceit_status === 'unavailable'" class="text-inverse text-opacity-50">
										Stats Faceit indisponibles.
									</div>
									<div v-else-if="profile.faceit_nickname">
										<div class="d-flex align-items-center mb-4 gap-3">
											<div>
												<div class="fw-bold fs-16px">{{ profile.faceit_nickname }}</div>
												<a v-if="profile.faceit_url" :href="profile.faceit_url.replace('{lang}', 'fr')" target="_blank" class="text-inverse text-opacity-50 fs-12px text-decoration-none">
													Voir le profil Faceit →
												</a>
											</div>
											<div class="ms-auto text-center">
												<div class="fs-28px fw-bold text-theme">{{ profile.faceit_elo }}</div>
												<div class="fs-11px text-inverse text-opacity-50">ELO</div>
											</div>
											<div class="text-center">
												<div class="fs-28px fw-bold" :class="`text-faceit-level-${profile.faceit_level}`">
													{{ profile.faceit_level }}
												</div>
												<div class="fs-11px text-inverse text-opacity-50">Niveau</div>
											</div>
										</div>

										<div class="row g-3" v-if="profile.faceit_matches">
											<div class="col-6 col-md-3">
												<div class="bg-inverse bg-opacity-10 rounded p-3 text-center">
													<div class="fs-20px fw-bold text-theme">{{ profile.faceit_matches }}</div>
													<div class="fs-12px text-inverse text-opacity-50">Matchs</div>
												</div>
											</div>
											<div class="col-6 col-md-3">
												<div class="bg-inverse bg-opacity-10 rounded p-3 text-center">
													<div class="fs-20px fw-bold text-theme">{{ profile.faceit_win_rate }}%</div>
													<div class="fs-12px text-inverse text-opacity-50">Win Rate</div>
												</div>
											</div>
											<div class="col-6 col-md-3">
												<div class="bg-inverse bg-opacity-10 rounded p-3 text-center">
													<div class="fs-20px fw-bold text-theme">{{ profile.faceit_kd_ratio }}</div>
													<div class="fs-12px text-inverse text-opacity-50">K/D</div>
												</div>
											</div>
											<div class="col-6 col-md-3">
												<div class="bg-inverse bg-opacity-10 rounded p-3 text-center">
													<div class="fs-20px fw-bold text-theme">{{ profile.faceit_headshots }}%</div>
													<div class="fs-12px text-inverse text-opacity-50">Headshots</div>
												</div>
											</div>
										</div>
									</div>
									<div v-else-if="profile.faceit_status === 'not_found'" class="text-inverse text-opacity-50">
										Aucun compte Faceit associé à ce profil Steam.
									</div>
								</div>

								<!-- Matchmaking -->
								<div class="tab-pane fade" id="tab-matchmaking">
									<p class="text-inverse text-opacity-50">
										Les statistiques de matchmaking apparaîtront ici une fois les premières parties jouées.
									</p>
								</div>

							</div>
						</div>
						<!-- END content -->

					</div>
				</div>
			</card-body>
		</card>
	</div>
</template>
