<script lang="ts">
	import '../app.css';
	import 'sanitize.css';
	import 'material-symbols/';
	import { Theme } from '@ui';
	import GlobalNav from '$lib/components/GlobalNavigationBar.svelte';
	import { ZarfPalettes } from '$lib/palette';
</script>

<svelte:head>
	<title>Zarf UI</title>
</svelte:head>
<Theme palettes={ZarfPalettes}>
	<GlobalNav />
	<main>
		<slot />
	</main>
</Theme>

<style>
	/* Roboto normal (400) */
	@import '@fontsource/roboto';
	@import '@fontsource/roboto/500';
	@import '@fontsource/roboto/300';
</style>
