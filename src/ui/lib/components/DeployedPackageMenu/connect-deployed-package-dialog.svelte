<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import type { ConnectString, DeployedPackage } from '$lib/api-types';
	import { IconButton, List, Typography, type SSX } from '@ui';
	import ZarfDialog from '../zarf-dialog.svelte';
	import Spinner from '../spinner.svelte';
	import { Packages, Tunnels } from '$lib/api';
	import ButtonDense from '../button-dense.svelte';
	import { tunnelStore } from '$lib/store';
	import { onDestroy } from 'svelte/internal';
	export let pkg: DeployedPackage;
	export let toggleDialog: () => void;

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

	async function getPackageConnections(): Promise<
		{ name: string; connectString: ConnectString }[]
	> {
		const result = await Packages.packageConnections(pkg.name);

		const connections = Object.entries(result.connectStrings).map(([name, connectString]) => ({
			name,
			connectString,
		}));
		selectedConnection = connections[0].name || '';
		return connections;
	}

	function addTunnel(pkgName: string, connectionName: string) {
		const tunnels = { ...$tunnelStore };
		if (!tunnels[pkgName]) {
			tunnels[pkgName] = [connectionName];
		} else {
			const connections = tunnels[pkgName];
			if (!connections.includes(connectionName)) {
				connections.push(connectionName);
			}
		}
		tunnelStore.set(tunnels);
	}

	async function connectPackage(): Promise<void> {
		try {
			await Tunnels.connect(selectedConnection);
			addTunnel(pkg.name, selectedConnection);
			toggleDialog();
		} catch (err: any) {
			errMessage = err.message;
		}
	}

	onDestroy(() => {
		errMessage = '';
	});

	$: failedToConnect = errMessage !== '';
	$: titleText =
		(failedToConnect && `Failed to connect to ${selectedConnection}`) || 'Connect to Resource';
</script>

<ZarfDialog bind:toggleDialog {titleText} happyZarf={!failedToConnect}>
	<svelte:fragment>
		{#if !failedToConnect}
			<Typography variant="body1" color="text-secondary-on-dark">
				Select which resource you would like Zarf to connect to. Zarf will create a secure tunnel
				and open the connection in a new tab
			</Typography>
			{#await getPackageConnections()}
				<Spinner />
			{:then connections}
				<List ssx={listSSX}>
					{#each connections as connection}
						<Typography
							variant="body1"
							element="li"
							value={connection.name}
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
							Zarf Connect {connection.name}
						</Typography>
					{/each}
				</List>
			{/await}
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
