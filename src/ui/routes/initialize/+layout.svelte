<script lang="ts">
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import Container from '$lib/components/container.svelte';
	import { pkgStore } from '$lib/store';
	import { Stepper } from '@ui';

	Packages.readInit().then(pkgStore.set);
</script>

<Container>
	<Stepper
		orientation="horizontal"
		steps={[
			{
				title: 'Configure',
				iconContent: $page.routeId == 'initialize/configure' ? '1' : undefined,
				disabled: false,
				variant: 'primary'
			},
			{
				title: 'Review',
				iconContent: $page.routeId == 'initialize/review' ? '2' : undefined,
				disabled: $page.routeId == 'initialize/configure',
				variant: 'primary'
			},
			{
				title: 'Deploy',
				iconContent: '3',
				disabled: $page.routeId != 'initialize/deploy',
				variant: 'primary'
			}
		]}
	/>

	{#if $pkgStore}
		<slot />
	{/if}
</Container>

<style>
	h1 {
		font-size: 34px;
		font-weight: 400;
		line-height: 42px;
		letter-spacing: 0.25px;
	}
	h2 {
		display: flex;
		gap: 0.75rem; /* 12px */
	}
	:global(.actionButtonsContainer) {
		display: flex;
		justify-content: space-between;
		margin-top: 2rem;
	}

	:global(.component-accordion-header) {
		display: flex;
		justify-content: space-between;
		width: 100%;
	}
	:global(.accordion-header) {
		width: 100%;
	}
</style>
