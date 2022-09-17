<script lang="ts">
	import Header from '$lib/components/header.svelte';
	import '@fontsource/roboto';
	import 'sanitize.css';
</script>

<Header />

<main>
	<slot />
</main>

<style>
	:global(html, body) {
		margin: 0;
		padding: 0;
		font-family: Roboto, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Oxygen, Ubuntu,
			Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
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
