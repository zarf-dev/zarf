<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import type { ConnectString, DeployedPackage } from '$lib/api-types';
	import { IconButton, List, Typography, type SSX } from '@ui';
	import ZarfDialog from '../zarf-dialog.svelte';
	import Spinner from '../spinner.svelte';
	import { Packages } from '$lib/api';
	import ButtonDense from '../button-dense.svelte';
	export let pkg: DeployedPackage;
	export let toggleDialog: () => void;

	let selectedConnection = '';

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
</script>

<ZarfDialog bind:toggleDialog titleText="Connect to Resource">
	<Typography variant="body1" color="text-secondary-on-dark">
		Select which resource you would like Zarf to connect to. Zarf will create a secure tunnel and
		open the connection in a new tab
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
	<svelte:fragment slot="actions">
		<ButtonDense on:click={toggleDialog} variant="outlined" backgroundColor="white">
			Cancel
		</ButtonDense>
		<ButtonDense variant="raised" backgroundColor="white" textColor="black" on:click={toggleDialog}>
			Connect
		</ButtonDense>
	</svelte:fragment>
</ZarfDialog>
