<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Stepper } from '@ui';
	import { page } from '$app/stores';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { PackageErrNotFound } from '$lib/components';

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

<section class="page">
	<div class="deploy-stepper-container">
		<Stepper
			orientation="horizontal"
			steps={[
				{
					title: 'Configure',
					iconContent: $page.route.id?.endsWith("/configure") ? '1' : undefined,
					variant: 'primary'
				},
				{
					title: 'Review',
					iconContent: !$page.route.id?.endsWith("/deploy") ? '2' : undefined,
					disabled: !$page.route.id?.endsWith("/review"),
					variant: 'primary'
				},
				{
					title: 'Deploy',
					iconContent: '3',
					disabled: !$page.route.id?.endsWith("/deploy"),
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
</style>
