<script lang="ts">
	import '../app.css';
	import 'sanitize.css';
	import 'material-symbols/';
	import { Theme } from '@ui';
	import GlobalNav from '$lib/components/global-navigation-bar.svelte';
	import { ZarfPalettes } from '$lib/palette';
	import NavDrawer from '$lib/components/nav-drawer.svelte';
	import { Cluster } from '$lib/api';
	import { clusterStore } from '$lib/store';

	function getClusterSummary() {
		// Try to get the cluster summary
		Cluster.summary()
			// If success update the store
			.then(clusterStore.set)
			// Otherwise, try again in 250 ms
			.catch((e) => {
				setTimeout(getClusterSummary, 250);
			});
	}

	getClusterSummary();
</script>

<svelte:head>
	<title>Zarf UI</title>
</svelte:head>
<Theme palettes={ZarfPalettes}>
	<GlobalNav />
	<main>
		<NavDrawer />
		<section class="page-content">
			<slot />
		</section>
	</main>
</Theme>

<style>
	/* Roboto normal (400) */
	@import '@fontsource/roboto';
	@import '@fontsource/roboto/500';
	@import '@fontsource/roboto/300';

	main {
		display: flex;
		width: 100vw;
		height: calc(100vh - 3.5rem);
		overflow: hidden;
	}
	.page-content {
		width: calc(100% - 16rem);
		height: 100%;
		overflow-y: auto;
		overflow-x: hidden;
		display: flex;
		flex-direction: column;
		padding: 2.5rem;
	}
</style>
