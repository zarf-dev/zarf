<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import RemoveDeployedPackageDialog from './remove-deployed-package-dialog.svelte';

	import { goto } from '$app/navigation';
	import { Kind, type DeployedPackage } from '$lib/api-types';
	import { IconButton, ListItem, ListItemAdornment, Menu, type SSX } from '@ui';
	import { current_component } from 'svelte/internal';
	import ConnectDeployedPackageDialog from './connect-deployed-package-dialog.svelte';

	export let pkg: DeployedPackage;

	let anchorRef: HTMLButtonElement;
	let toggleRemoveDialog: () => void;
	let toggleConnectDialog: () => void;

	let updateLink = '';
	let toggled = false;

	const menuSSX: SSX = {
		$self: {
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
	<ListItem text="Connect..." on:click={toggleConnectDialog} />
	<ListItem text="Update Package..." on:click={() => goto(updateLink)}>
		<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined">
			cached
		</ListItemAdornment>
	</ListItem>
	<div class="divider" />
	<ListItem text="Remove..." on:click={toggleRemoveDialog}>
		<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined">
			delete
		</ListItemAdornment>
	</ListItem>
</Menu>
<ConnectDeployedPackageDialog {pkg} bind:toggleDialog={toggleConnectDialog} />
<RemoveDeployedPackageDialog {pkg} bind:toggleDialog={toggleRemoveDialog} />
