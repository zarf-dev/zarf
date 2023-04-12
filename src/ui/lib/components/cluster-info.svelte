<script lang="ts">
	import ConnectClusterDialog from './connect-cluster-dialog.svelte';
	import ClusterInfoTable from './cluster-info-table.svelte';
	import { fade } from 'svelte/transition';
	import { Paper, Typography, Box, type SSX } from '@ui';
	import ButtonDense from './button-dense.svelte';
	import { clusterStore } from '$lib/store';
	import Spinner from './spinner.svelte';

	let toggleDialog: () => void;

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			'& .cluster-info-body': {
				minHeight: '86px',
				maxHeight: '160px',
				borderTopLeftRadius: '0px',
				borderTopRightRadius: '0px',
				'& > .cluster-not-connected': {
					display: 'flex',
					height: '100%',
					alignItems: 'center',
					justifyContent: 'center',
					gap: '8px',
				},
			},
			'& .cluster-info-header': {
				padding: '16px',
				height: '56px',
				display: 'flex',
				alignItems: 'center',
				justifyContent: 'space-between',
				borderBottomRightRadius: '0px',
				borderBottomLeftRadius: '0px',
			},
		},
	};

	$: showClusterInfo = $clusterStore?.hasZarf;
</script>

<Box {ssx} class="cluster-info-container">
	<Paper class="cluster-info-header" elevation={1}>
		<div class="header-right">
			<Typography variant="th">Cluster</Typography>
		</div>
		<div class="header-left">
			{#if !showClusterInfo}
				<ButtonDense variant="outlined" backgroundColor="white" on:click={toggleDialog}
					>Connect Cluster</ButtonDense
				>
			{/if}
		</div>
	</Paper>
	<Paper class="cluster-info-body" elevation={1}>
		{#if !$clusterStore}
			<div class="cluster-not-connected" in:fade={{ duration: 1000 }}>
				<Typography variant="body1" color="blue-200">Searching for cluster.</Typography>
				<Spinner color="blue-200" />
			</div>
		{:else if !showClusterInfo}
			<Typography class="cluster-not-connected" variant="body1" element="span">
				<span class="material-symbols-outlined" style="color:var(--warning);"> warning </span>
				<span>&nbsp;Cluster not connected </span>
			</Typography>
		{:else}
			<ClusterInfoTable />
		{/if}
	</Paper>
	<ConnectClusterDialog bind:toggleDialog />
</Box>
