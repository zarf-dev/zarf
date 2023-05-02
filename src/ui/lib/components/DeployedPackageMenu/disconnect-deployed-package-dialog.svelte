<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { IconButton, List, Typography, type SSX } from '@ui';
	import type { DeployedPackage } from '$lib/api-types';
	import ButtonDense from '../button-dense.svelte';
	import ZarfDialog from '../zarf-dialog.svelte';
	import { onDestroy } from 'svelte/internal';
	import { Packages } from '$lib/api';
	import { tunnelStore, updateConnections } from '$lib/store';

	export let pkg: DeployedPackage;
	export let toggleDialog: () => void;

	let open: boolean;
	let selectedConnection = '';
	let errMessage: string = '';

	const listSSX: SSX = {
		$self: {
			maxHeight: '100px',
			overflowY: 'scroll',
			padding: '0px',
			'& > li': {
				display: 'flex',
				alignItems: 'center',
				padding: '0px',
			},
			'& .icon-button': {
				width: '38px !important',
				height: '38px !important',
				'&:hover': {
					background: 'var(--shades-primary-16p)',
				},
				color: 'var(--primary)',
				'& .material-symbols-outlined': {
					fontSize: '17px',
				},
			},
		},
	};

	async function disconnect(): Promise<void> {
		try {
			await Packages.disconnect(pkg.name, selectedConnection);
			await updateConnections();
			toggleDialog();
		} catch (err: any) {
			errMessage = err.message;
		}
	}

	onDestroy(() => {
		errMessage = '';
	});
	$: connections = $tunnelStore[pkg.name] || [];
	$: failedToDisconnect = errMessage !== '';
	$: titleText =
		(failedToDisconnect && `Failed to disconnect ${selectedConnection}`) || 'Disconnect Resource';
	$: {
		if (open && connections.length > 0) {
			selectedConnection = connections[0].name;
		}
	}
</script>

<ZarfDialog bind:open bind:toggleDialog {titleText} happyZarf={!failedToDisconnect}>
	<svelte:fragment>
		{#if !failedToDisconnect}
			<Typography variant="body1" color="text-secondary-on-dark">
				Select which resource you would like Zarf to disconnect from. Zarf will close and remove the
				secure tunnel.
			</Typography>
			{#if connections.length > 0}
				<List ssx={listSSX}>
					{#each connections as connection}
						<Typography
							variant="body1"
							element="li"
							value={connection}
							on:click={() => (selectedConnection = connection.name)}
						>
							<IconButton
								toggleable
								toggled={selectedConnection === connection.name}
								iconClass="material-symbols-outlined"
								iconContent="radio_button_unchecked"
								iconColor="primary"
								toggledIconClass="material-symbols-outlined"
								toggledIconContent="radio_button_checked"
							/>
							Zarf Disconnect {connection.name}
						</Typography>
					{/each}
				</List>
			{:else}
				<Typography variant="body1" color="text-secondary-on-dark">
					No resources to disconnect
				</Typography>
			{/if}
		{:else}
			<Typography variant="body1" color="text-secondary-on-dark">
				{errMessage}
			</Typography>
		{/if}
	</svelte:fragment>
	<svelte:fragment slot="actions">
		<ButtonDense on:click={toggleDialog} variant="outlined" backgroundColor="white">
			Cancel
		</ButtonDense>
		{#if !failedToDisconnect}
			<ButtonDense variant="raised" backgroundColor="white" textColor="black" on:click={disconnect}>
				Disconnect
			</ButtonDense>
		{/if}
	</svelte:fragment>
</ZarfDialog>
