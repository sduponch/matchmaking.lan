<script>
export default {
	mounted() {
		const token = new URLSearchParams(window.location.search).get('token');

		if (token && window.opener) {
			window.opener.postMessage({ token }, window.location.origin);
		}

		window.close();
	}
}
</script>

<template>
	<div></div>
</template>
