<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Stepper } from '@ui';
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
  	import { ErrNotFound } from '$lib/components/package';

	enum LoadingStatus {
		Loading,
		Success,
		Error
	}

	let status: LoadingStatus = LoadingStatus.Loading;
	let errMessage: string = '';

	Packages.findInit()
		.catch(async (err: Error) => {
			errMessage = err.message;
			status = LoadingStatus.Error;
		})
		.then((res) => {
			if (Array.isArray(res)) {
				Packages.read(res[0]).then(pkgStore.set);
				status = LoadingStatus.Success;
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
	{#if status == LoadingStatus.Loading}
		<!-- placeholder loading content -->
		<div>loading...</div>
	{:else if status == LoadingStatus.Error}
		<ErrNotFound pkgName={errMessage.split(':')[1]} />
	{:else}
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
	{/if}
</section>
