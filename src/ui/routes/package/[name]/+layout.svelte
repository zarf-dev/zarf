<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Stepper, Typography } from '@ui';
	import { page } from '$app/stores';
	import { pkgComponentDeployStore, pkgStore, clusterStore } from '$lib/store';

	import { goto } from '$app/navigation';

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
	if (!$pkgStore) {
		goto('/', { replaceState: true });
	}
</script>

{#if $clusterStore.hasZarf == false && $pkgStore.zarfPackage.kind != "ZarfInitConfig"}
	<div class="warning-banner">
		<Typography variant="body1"
			>WARNING: You are deploying a package without an initialized Zarf cluster</Typography
		>
	</div>
{/if}

<section class="page">
	<div class="deploy-stepper-container">
		<Stepper
			orientation="horizontal"
			steps={[
				{
					title: 'Configure',
					iconContent: $page.route.id?.endsWith('/configure') ? '1' : undefined,
					variant: 'primary'
				},
				{
					title: 'Review',
					iconContent: !$page.route.id?.endsWith('/deploy') ? '2' : undefined,
					disabled: !$page.route.id?.endsWith('/review'),
					variant: 'primary'
				},
				{
					title: 'Deploy',
					iconContent: '3',
					disabled: !$page.route.id?.endsWith('/deploy'),
					variant: 'primary'
				}
			]}
		/>
	</div>
	{#if $pkgStore}
		<slot />
	{/if}
</section>

<style>
	.deploy-stepper-container {
		max-width: 600px;
		margin: 0 auto;
		width: 100%;
	}
	/* remove when UnicornUI updates w/ fix */
	:global(.deploy-stepper-container ol) {
		padding-inline: 0;
	}
	:global(.deploy-stepper-container li:last-child) {
		flex-grow: 0;
	}
	.warning-banner {
		width: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		background-color: var(--uui-default-colors-warning);
		margin-top: 1rem;
		padding: 1rem;
		position: sticky;
		top: 0;
	}
</style>
