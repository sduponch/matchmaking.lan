<script>
import { useAppOptionStore } from '@/stores/app-option';
import config from '@/config/matchmaking';

const appOption = useAppOptionStore();

export default {
	mounted() {
		appOption.appSidebarHide = true;
		appOption.appHeaderHide = true;
		appOption.appContentClass = 'p-0';

		window.addEventListener('message', this.onSteamMessage);
	},
	beforeUnmount() {
		appOption.appSidebarHide = false;
		appOption.appHeaderHide = false;
		appOption.appContentClass = '';

		window.removeEventListener('message', this.onSteamMessage);
	},
	methods: {
		loginWithSteam() {
			const params = new URLSearchParams({
				'openid.ns':         'http://specs.openid.net/auth/2.0',
				'openid.mode':       'checkid_setup',
				'openid.return_to':  `${config.api.baseUrl}/auth/steam`,
				'openid.realm':      config.api.baseUrl,
				'openid.identity':   'http://specs.openid.net/auth/2.0/identifier_select',
				'openid.claimed_id': 'http://specs.openid.net/auth/2.0/identifier_select',
			});

			const width = 500;
			const height = 620;
			const left = window.screenX + (window.outerWidth - width) / 2;
			const top  = window.screenY + (window.outerHeight - height) / 2;

			window.open(
				`https://steamcommunity.com/openid/login?${params}`,
				'steam-login',
				`width=${width},height=${height},left=${left},top=${top},resizable=no`
			);
		},
		onSteamMessage(event) {
			if (event.origin !== window.location.origin) return;
			if (!event.data?.token) return;

			localStorage.setItem('token', event.data.token);
			this.$router.push('/home');
		}
	}
}
</script>

<template>
	<!-- BEGIN login -->
	<div class="login">
		<!-- BEGIN login-content -->
		<div class="login-content">
			<h1 class="text-center mb-1">matchmaking.lan</h1>
			<div class="text-inverse text-opacity-50 text-center mb-5">
				Plateforme de matchmaking CS2
			</div>

			<button
				type="button"
				class="btn btn-outline-theme btn-lg d-flex align-items-center justify-content-center gap-3 w-100 fw-500"
				v-on:click="loginWithSteam"
			>
				<img src="@/assets/steam-icon.svg" alt="Steam" width="24" height="24" />
				Se connecter avec Steam
			</button>

			<div class="text-center text-inverse text-opacity-50 mt-4" style="font-size: .8rem">
				Seuls les joueurs enregistrés par un administrateur peuvent accéder à la plateforme.
			</div>
		</div>
		<!-- END login-content -->
	</div>
	<!-- END login -->
</template>
