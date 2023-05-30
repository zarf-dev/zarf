<!--
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { Kind, type DeployedPackage } from '$lib/api-types';
	import { IconButton, ListItem, ListItemAdornment, Menu, Typography, type SSX } from '@ui';
	import { current_component } from 'svelte/internal';
	import { tunnelStore } from '$lib/store';
	import RemoveDeployedPackageDialog from './remove-deployed-package-dialog.svelte';
	import ConnectDeployedPackageDialog from './connect-deployed-package-dialog.svelte';
	import DisconnectDeployedPackageDialog from './disconnect-deployed-package-dialog.svelte';

	export let pkg: DeployedPackage;

	let anchorRef: HTMLButtonElement;
	let toggleRemoveDialog: () => void;
	let toggleConnectDialog: () => void;
	let toggleDisconnectDialog: () => void;

	let updateLink = '';
	let toggled = false;

	const menuSSX: SSX = {
		$self: {
			'position': 'fixed',
			'& .list-item-adornment': { color: 'var(--text-secondary-on-dark)' },
			'& .divider': { height: '1px', boxShadow: 'inset 0px -1px 0px rgba(255,255,255,0.12)' },
		},
	};

	function handleClick(event: any) {
		if (anchorRef && !anchorRef.contains(event.target) && !event.defaultPrevented) {
			if (anchorRef !== event.target) {
				toggled = false;
			}
		}
	}

	$: {
		if (pkg.data.kind === Kind.ZarfInitConfig) {
			updateLink = '/packages?init=true';
		} else {
			updateLink = '/packages';
		}
	}

	$: hasConnections = $tunnelStore[pkg.name]?.length > 0;
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
	{#if hasConnections}
		<ListItem on:click={toggleDisconnectDialog}>
			<ListItemAdornment slot="leading" class="material-symbols-outlined">
				signal_disconnected
			</ListItemAdornment>
			<Typography>
				Disconnect...
			</Typography>
		</ListItem>
	{/if}
	<ListItem on:click={toggleConnectDialog}>
		<ListItemAdornment slot="leading" class="material-symbols-outlined">
			private_connectivity
		</ListItemAdornment>
		<Typography>
			Connect...
		</Typography>
	</ListItem>
	<ListItem on:click={() => goto(updateLink)}>
		<ListItemAdornment slot="leading" class="material-symbols-outlined">
			cached
		</ListItemAdornment>
		<Typography>
			Update Package...
		</Typography>
	</ListItem>
	<div class="divider" />
	<ListItem on:click={toggleRemoveDialog}>
		<ListItemAdornment slot="leading" class="material-symbols-outlined">
			delete
		</ListItemAdornment>
		<Typography>
			Remove...
		</Typography>
	</ListItem>
</Menu>
<DisconnectDeployedPackageDialog {pkg} bind:toggleDialog={toggleDisconnectDialog} />
<ConnectDeployedPackageDialog {pkg} bind:toggleDialog={toggleConnectDialog} />
<RemoveDeployedPackageDialog {pkg} bind:toggleDialog={toggleRemoveDialog} />
