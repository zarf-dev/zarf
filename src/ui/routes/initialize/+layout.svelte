<script lang="ts">
	import { Stepper } from '@ui';
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';

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

<section class="initStepPage">
	<Stepper
		orientation="horizontal"
		steps={[
			{
				title: 'Configure',
				iconContent: $page.routeId === 'initialize/configure' ? '1' : undefined,
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
</section>

<style>
	.initStepPage {
		padding: 2rem 10rem;
		display: flex;
		flex-direction: column;
		gap: 2rem;
	}
	@media (max-width: 900px) {
		.initStepPage {
			padding: 2rem 4rem;
		}
	}
	@media (max-width: 600px) {
		.initStepPage {
			padding: 2rem 1rem;
		}
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
