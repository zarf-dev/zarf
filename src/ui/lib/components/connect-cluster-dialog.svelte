<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Typography, Box, Button, type SSX, Dialog, List, IconButton, DialogActions } from '@ui';
	import { clusterStore } from '$lib/store';
	import ZarfDialog from './zarf-dialog.svelte';

	export let toggleDialog: () => void;

	let titleText: string;
	let zarfAlt: string;
	let happyZarf: boolean;

	const ssx: SSX = {
		$self: {
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
	$: hasDistro = $clusterStore?.distro;
	$: {
		if (hasDistro) {
			titleText = 'Kubeconfig Found';
			happyZarf = true;
			zarfAlt = 'Found a cluster.';
		} else {
			titleText = 'Kubeconfig Not Found';
			happyZarf = false;
			zarfAlt = 'Failed to find a cluster.';
		}
	}
</script>

<ZarfDialog bind:toggleDialog {titleText} {happyZarf} {zarfAlt}>
	{#if hasDistro}
		<Box
			ssx={{
				$self: {
					marginTop: '-8px',
					display: 'flex',
					flexDirection: 'column',
					gap: '8px',
				},
			}}
		>
			<Typography variant="body2" color="text-secondary-on-dark">
				Which cluster would you like Zarf to connect to?
			</Typography>
			<List {ssx}>
				<Typography variant="body1" element="li" value={$clusterStore?.distro}>
					<IconButton
						toggleable
						toggled
						iconClass="material-symbols-outlined"
						iconContent="radio_button_unchecked"
						iconColor="primary"
						toggledIconClass="material-symbols-outlined"
						toggledIconContent="radio_button_checked"
					/>
					{$clusterStore?.rawConfig['current-context']}
				</Typography>
			</List>
			<Typography variant="caption" color="text-secondary-on-dark">
				Clusters can be managed in your system Kubconfig file.
			</Typography>
		</Box>
	{:else}
		<Typography variant="caption" color="text-secondary-on-dark">
			Zarf requires access to a cluster. Please spin up a cluster or log in to your cluster
			provider, then try connecting to the cluster again.
		</Typography>
	{/if}
	<svelte:fragment slot="actions">
		{#if hasDistro}
			<Button on:click={toggleDialog} variant="outlined" backgroundColor="grey-300">cancel</Button>
			<Button
				href="/packages?init=true"
				variant="raised"
				backgroundColor="grey-300"
				textColor="text-primary-on-light"
			>
				Connect Cluster
			</Button>
		{:else}
			<Button
				on:click={toggleDialog}
				variant="raised"
				backgroundColor="grey-300"
				textColor="text-primary-on-light"
			>
				Close
			</Button>
		{/if}
	</svelte:fragment>
</ZarfDialog>
