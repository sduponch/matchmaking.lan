<script>
export default {
	mounted() {
		this.$refs.link.click();
	}
}
</script>

<template>
	<div class="d-flex justify-content-center align-items-center" style="min-height: 300px">
		<div class="text-center">
			<a ref="link" :href="`steam://connect/${$route.params.addr}`" class="d-none"></a>
			<div class="mb-4">
				<div class="spinner-border text-theme mb-3" role="status"></div>
				<div class="text-inverse text-opacity-75">Connexion au serveur en cours…</div>
			</div>
			<a :href="`steam://connect/${$route.params.addr}`" class="btn btn-outline-theme btn-sm">
				Cliquez ici si la connexion ne s'ouvre pas
			</a>
		</div>
	</div>
</template>
