<script lang="ts">
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import Container from '$lib/components/container.svelte';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Stepper } from '@ui';

	Packages.findInit().then((initPackages) => {
		if (initPackages.length > 0) {
			Packages.read(initPackages[0]).then(pkgStore.set);
		}
	});

	let setupComplete = false;

	pkgStore.subscribe((pkg) => {
		if (!setupComplete && pkg) {
			let selected: number[] = [];
			pkg.zarfPackage.components.forEach((component, index) => {
				if (component.required) {
					selected.push(index);
				}
			});

			// Update the store with the required components
			pkgComponentDeployStore.set(selected);

			setupComplete = true;
		}
	});
</script>

<Container>
	<Stepper
		orientation="horizontal"
		steps={[
			{
				title: 'Configure',
				iconContent: $page.routeId === 'initialize/configure' ? '1' : undefined,
				disabled: false,
				variant: 'primary'
			},
			{
				title: 'Review',
				iconContent: $page.routeId !== 'initialize/deploy' ? '2' : undefined,
				disabled: $page.routeId === 'initialize/configure',
				variant: 'primary'
			},
			{
				title: 'Deploy',
				iconContent: '3',
				disabled: $page.routeId !== 'initialize/deploy',
				variant: 'primary'
			}
		]}
	/>

	{#if $pkgStore}
		<slot />
	{/if}
</Container>

<style>
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
