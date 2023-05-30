<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { IconButton, List, Typography, type SSX } from '@ui';
	import type { DeployedPackage } from '$lib/api-types';
	import { onDestroy, onMount } from 'svelte/internal';
	import ButtonDense from '../button-dense.svelte';
	import ZarfDialog from '../zarf-dialog.svelte';
	import { updateConnections } from '$lib/store';
	import { Packages } from '$lib/api';

	// Props
	export let pkg: DeployedPackage;
	export let toggleDialog: () => void;

	// Locals
	let open: boolean;
	let currentWindow: Window;
	let selectedConnection = '';
	let errMessage: string = '';
	const connections: string[] = Object.keys(pkg.connectStrings || {}) || [];

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

	async function connectPackage(): Promise<void> {
		try {
			const connection = await Packages.connect(pkg.name, selectedConnection);
			currentWindow.open(connection.url, '_blank');
			await updateConnections();
			toggleDialog();
		} catch (err: any) {
			errMessage = err.message;
		}
	}

	onMount(() => {
		currentWindow = window;
	});

	onDestroy(() => {
		errMessage = '';
	});

	$: failedToConnect = errMessage !== '';
	$: titleText =
		(failedToConnect && `Failed to connect to ${selectedConnection}`) || 'Connect to Resource';
	$: {
		if (open && connections.length > 0) {
			selectedConnection = connections[0];
		}
	}
</script>

<ZarfDialog bind:open bind:toggleDialog {titleText} happyZarf={!failedToConnect}>
	<svelte:fragment>
		{#if !failedToConnect}
			{#if connections.length === 0}
				<Typography variant="body1" color="text-secondary-on-dark">
					No connections available for this package
				</Typography>
			{:else}
				<Typography variant="body1" color="text-secondary-on-dark">
					Select which resource you would like Zarf to connect to. Zarf will create a tunnel and
					open the connection in a new tab
				</Typography>
				<List ssx={listSSX} backgroundColor="transparent">
					{#each connections as connection}
						<Typography
							variant="body1"
							element="li"
							value={connection}
							on:click={() => (selectedConnection = connection)}
						>
							<IconButton
								toggleable
								toggled={selectedConnection === connection}
								iconClass="material-symbols-outlined"
								iconContent="radio_button_unchecked"
								iconColor="primary"
								toggledIconClass="material-symbols-outlined"
								toggledIconContent="radio_button_checked"
							/>
							Zarf Connect {connection}
						</Typography>
					{/each}
				</List>
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
		{#if !failedToConnect}
			<ButtonDense
				variant="raised"
				backgroundColor="white"
				textColor="black"
				on:click={connectPackage}
			>
				Connect
			</ButtonDense>
		{/if}
	</svelte:fragment>
</ZarfDialog>
