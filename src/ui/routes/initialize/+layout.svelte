<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
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

<section class="page">
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
