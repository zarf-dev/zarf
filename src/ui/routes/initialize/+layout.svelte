<script lang="ts">
	import { Button, Stepper, Typography } from '@ui';
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';

	enum LoadingStatus {
		Loading,
		Success,
		Error
	}

	let status: LoadingStatus = LoadingStatus.Loading;
	let errMessage: string = '';

	Packages.findInit()
		.catch(async (err) => {
			if (err.status == 500) {
				errMessage = await err.data.text();
				status = LoadingStatus.Error;
			}
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
		<!-- replace w/ error dialog -->
		<div class="center">
			<Typography variant="h1">Package Not Found</Typography>
			<Typography variant="body2">
				Make sure the following package is in the current working directory:
			</Typography>
			<Typography variant="code">{errMessage}</Typography>
			<Button href="/" color="secondary" style="margin-top: 0.5rem;" variant="flat"
				>Return Home</Button
			>
		</div>
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

<style>
	.center {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1rem;
	}
</style>
