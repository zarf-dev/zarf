<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { Kind, type DeployedPackage } from '$lib/api-types';
	import {
		IconButton,
		ListItem,
		ListItemAdornment,
		Menu,
		Typography,
		TextField,
		type SSX,
	} from '@ui';
	import { current_component } from 'svelte/internal';
	import { Packages } from '$lib/api';
	import ZarfDialog from './zarf-dialog.svelte';
	import ButtonDense from './button-dense.svelte';
	import Spinner from './spinner.svelte';

	export let pkg: DeployedPackage;

	let anchorRef: HTMLButtonElement;
	let toggleDialog: () => void;
	let inputValue: string;
	let errorMessage: string;
	let happyZarf: boolean;
	let titleText: string;
	let zarfAlt: string;

	let updateLink = '';
	let toggled = false;
	let removing = false;

	const menuSSX: SSX = {
		$self: {
			'& .list-item-adornment': { color: 'var(--text-secondary-on-dark)' },
			'& .divider': { height: '1px', boxShadow: 'inset 0px -1px 0px rgba(255,255,255,0.12)' },
		},
	};

	function closeDialog() {
		inputValue = '';
		errorMessage = '';
		removing = false;
		toggleDialog();
	}

	function handleClick(event: any) {
		if (anchorRef && !anchorRef.contains(event.target) && !event.defaultPrevented) {
			if (anchorRef !== event.target) {
				toggled = false;
			}
		}
	}

	async function removePkg(): Promise<void> {
		removing = true;
		Packages.remove(pkg.name)
			.then(closeDialog)
			.catch((e) => {
				errorMessage = e.message;
				removing = false;
			});
	}

	$: {
		if (pkg.data.kind === Kind.ZarfInitConfig) {
			updateLink = '/packages?init=true';
		} else {
			updateLink = '/packages';
		}
	}
	$: {
		if (!errorMessage) {
			happyZarf = true;
			titleText = 'Remove Package from Cluster';
			zarfAlt = 'Succeeded in removing a package from the cluster.';
		} else {
			happyZarf = false;
			titleText = `Failed to remove package ${pkg.name}`;
			zarfAlt = 'Failed to remove a package from the cluster.';
		}
	}
</script>

<svelte:window on:click={handleClick} />
<IconButton
	eventComponent={current_component}
	bind:ref={anchorRef}
	toggleable
	bind:toggled
	on:click={() => (toggled = !toggled)}
	iconClass="material-symbols-outlined"
	iconContent="more_vert"
/>
<Menu ssx={menuSSX} bind:anchorRef open={toggled} anchorOrigin="bottom-end">
	<ListItem text="Update Package..." on:click={() => goto(updateLink)}>
		<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined">
			cached
		</ListItemAdornment>
	</ListItem>
	<div class="divider" />
	<ListItem text="Remove..." on:click={toggleDialog}>
		<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined">
			delete
		</ListItemAdornment>
	</ListItem>
</Menu>
<ZarfDialog clickAway={!removing} bind:toggleDialog {happyZarf} {titleText} {zarfAlt}>
	{#if !errorMessage}
		{#if removing}
			<div style="display: flex; justify-content: center; align-items: center: width: 100%">
				<Spinner color="blue-200" diameter="50px" />
			</div>
		{:else}
			<Typography variant="body2" color="text-secondary-on-dark">
				Type the name of the package and click remove to delete the package and all of it’s
				resources from the cluster. This action step cannot be undone.
			</Typography>
			<Typography variant="subtitle1">{pkg.name}</Typography>
			<TextField
				variant="outlined"
				label="Package to Delete"
				color="primary"
				bind:value={inputValue}
				helperText="Type the name of the package."
			/>
		{/if}
	{:else}
		<Typography variant="body2" color="text-secondary-on-dark">
			{errorMessage}
		</Typography>
	{/if}
	<svelte:fragment slot="actions">
		{#if !errorMessage}
			<ButtonDense backgroundColor="white" variant="outlined" on:click={closeDialog}>
				Cancel
			</ButtonDense>
			<ButtonDense
				disabled={pkg.name !== inputValue}
				variant="flat"
				textColor="text-primary-on-light"
				backgroundColor="grey-300"
				on:click={removePkg}
			>
				Remove Package
			</ButtonDense>
		{:else}
			<ButtonDense backgroundColor="white" variant="outlined" on:click={closeDialog}>
				Close
			</ButtonDense>
		{/if}
	</svelte:fragment>
</ZarfDialog>
