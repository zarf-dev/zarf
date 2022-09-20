<script lang="ts">
	import { Cluster } from '$lib/api';
	import Header from '$lib/components/header.svelte';
	import { clusterStore } from '$lib/store';
	import '@fontsource/roboto';
	import 'sanitize.css';

	Cluster.summary().then(clusterStore.set);
</script>

<svelte:head>
	<title>Zarf UI</title>
</svelte:head>

<Header />

<main>
	<slot />
</main>

<style>
	:global(html, body) {
		margin: 0;
		padding: 0;
		background-color: #f0f0f0;
		font-family: Roboto, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Oxygen, Ubuntu,
			Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
	}

	:global(.card) {
		background-color: #fff;
	}

	:global(.accordion) {
		border-radius: 4px;
		margin-bottom: 14px;
	}

	:global(.accordion .accordion-header-wrapper) {
		border-radius: 4px;
	}

	:global(.stepper-horizontal .stepper-item) {
		z-index: 100;
	}

	:global(:root) {
		--mdc-theme-primary: #4adede;
		--mdx-theme-secondary: #4053c2;
		--mdc-theme-on-primary: #171717;
		--mdc-theme-secondary: green;
		--mdc-theme-on-secondary: white;
		--mdc-theme-surface: white;
	}
</style>
