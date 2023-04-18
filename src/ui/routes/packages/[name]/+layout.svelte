<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Typography } from '@ui';
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

{#if !$clusterStore?.hasZarf && $pkgStore?.zarfPackage?.kind !== 'ZarfInitConfig'}
	<div class="warning-banner">
		<Typography variant="body1">
			WARNING: You are deploying a package without an initialized Zarf cluster
		</Typography>
	</div>
{/if}

{#if $pkgStore}
	<slot />
{/if}

<style>
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
