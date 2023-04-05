<script lang="ts">
	import GlobalNav from '$lib/components/global-navigation-bar.svelte';
	import NavDrawer from '$lib/components/nav-drawer.svelte';
	import { ZarfPalettes } from '$lib/palette';
	import { Theme } from '@ui';
	import 'material-symbols/';
	import 'sanitize.css';
	import '../app.css';
	import { ZarfTypography } from '$lib/typography';
</script>

<svelte:head>
	<title>Zarf UI</title>
</svelte:head>
<Theme palettes={ZarfPalettes} typography={ZarfTypography}>
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
		gap: 48px;
	}
</style>
