import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/',              component: () => import('../views/PageLogin.vue'),  meta: { public: true } },
    { path: '/home',          component: () => import('../views/Home.vue') },
    { path: '/admin/servers',                  component: () => import('../views/admin/AdminServersManage.vue'),    meta: { admin: true } },
    { path: '/admin/server/setup',             component: () => import('../views/admin/AdminServerSetup.vue'),       meta: { admin: true } },
    { path: '/admin/matchmaking/players',      component: () => import('../views/admin/AdminPlayers.vue'),           meta: { admin: true } },
    { path: '/admin/matchmaking/teams',        component: () => import('../views/admin/AdminMatchmakingTeams.vue'),  meta: { admin: true } },
    { path: '/admin/cs2',                        component: () => import('../views/admin/AdminCS2Config.vue'),          meta: { admin: true } },
    { path: '/admin/matchmaking/match-configs', component: () => import('../views/admin/AdminMatchConfigs.vue'),       meta: { admin: true } },
    { path: '/admin/matchmaking/encounters',    component: () => import('../views/admin/AdminEncounters.vue'),          meta: { admin: true } },
    { path: '/admin/matchmaking/tournaments',  component: () => import('../views/admin/AdminTournaments.vue'),       meta: { admin: true } },
    { path: '/tournaments',                    component: () => import('../views/Tournaments.vue') },
    { path: '/tournaments/matches',            component: () => import('../views/TournamentMatches.vue') },
    { path: '/matchmaking',                    component: () => import('../views/Matchmaking.vue') },
    { path: '/profile',       component: () => import('../views/Profile.vue') },
    { path: '/auth/done',     component: () => import('../views/PageAuthDone.vue'), meta: { public: true } },
    { path: '/connect/:addr', component: () => import('../views/PageConnect.vue'), meta: { public: true } },
    { path: '/:pathMatch(.*)*', component: () => import('../views/PageError.vue'), meta: { public: true } },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  auth.init()

  // Utilisateur authentifié sur la page de login → dashboard
  if (to.path === '/' && auth.isAuthenticated) {
    return { path: '/home' }
  }

  // Route protégée sans JWT valide → login
  if (!to.meta.public && !auth.isAuthenticated) {
    return { path: '/' }
  }

  // Route admin sans rôle admin → accueil
  if (to.meta.admin && auth.user?.role !== 'admin') {
    return { path: '/home' }
  }
})

export default router
