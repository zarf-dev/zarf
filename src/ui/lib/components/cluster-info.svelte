<script lang="ts">
	import ClusterInfoTable from './cluster-info-table.svelte';

	import { Paper, Typography, Box, type SSX } from '@ui';
	import ButtonDense from './button-dense.svelte';
	import { clusterStore } from '$lib/store';

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			'& .cluster-info-body': {
				minHeight: '86px',
				maxHeight: '160px',
				'& > .cluster-not-connected': {
					display: 'flex',
					height: '100%',
					alignItems: 'center',
					justifyContent: 'center',
				},
			},
			'& .cluster-info-header': {
				padding: '16px',
				height: '56px',
				display: 'flex',
				alignItems: 'center',
				justifyContent: 'space-between',
			},
		},
	};
	$: showClusterInfo = $clusterStore?.hasZarf;
</script>

<Box {ssx} class="cluster-info-container">
	<Paper class="cluster-info-header" square elevation={1}>
		<div class="header-right">
			<Typography variant="th">Cluster</Typography>
		</div>
		<div class="header-left">
			{#if !showClusterInfo}
				<ButtonDense variant="outlined" backgroundColor="white">Connect Cluster</ButtonDense>
			{/if}
		</div>
	</Paper>
	<Paper class="cluster-info-body" square elevation={1}>
		{#if !showClusterInfo}
			<Typography class="cluster-not-connected" variant="body1" element="span">
				<span class="material-symbols-outlined" style="color:var(--warning);"> warning </span>
				<span>&nbsp;Cluster not connected </span>
			</Typography>
		{:else}
			<ClusterInfoTable />
		{/if}
	</Paper>
</Box>
