import { defineStore } from 'pinia'

interface User {
	steamid: string
	username: string
	avatar: string
	role: 'admin' | 'player'
}

function parseJWT(token: string): User | null {
	try {
		const payload = JSON.parse(atob(token.split('.')[1]))
		return {
			steamid:  payload.steamid,
			username: payload.username,
			avatar:   payload.avatar,
			role:     payload.role,
		}
	} catch {
		return null
	}
}

export const useAuthStore = defineStore('auth', {
	state: () => ({
		user: null as User | null,
		token: null as string | null,
	}),
	getters: {
		isAuthenticated: (state) => !!state.user,
		isAdmin: (state) => state.user?.role === 'admin',
	},
	actions: {
		init() {
			const token = localStorage.getItem('token')
			if (token) {
				const user = parseJWT(token)
				if (user) {
					this.token = token
					this.user = user
				} else {
					localStorage.removeItem('token')
				}
			}
		},
		setToken(token: string) {
			const user = parseJWT(token)
			if (!user) return
			this.token = token
			this.user = user
			localStorage.setItem('token', token)
		},
		logout() {
			this.token = null
			this.user = null
			localStorage.removeItem('token')
		}
	}
})
