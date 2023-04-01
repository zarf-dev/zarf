<script lang="ts">
	import { Paper, Typography } from '@ui';
	import ButtonDense from './button-dense.svelte';
	import { clusterStore } from '$lib/store';

	$: showClusterInfo = $clusterStore?.hasZarf;
</script>

<div class="cluster-info-container">
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
			<Typography
				variant="body1"
				element="span"
				style="display:flex;align-items:center;justify-content:center;"
			>
				<span class="material-symbols-outlined" style="color:var(--warning);"> warning </span>
				<span>&nbsp;Cluster not connected </span>
			</Typography>
		{/if}
	</Paper>
</div>

<style>
	.cluster-info-container {
		display: flex;
		flex-direction: column;
	}
	.cluster-info-container > :global(.paper) {
		padding: 16px;
	}

	.cluster-info-container :global(.cluster-info-body) {
		min-height: 86px;
		max-height: 160px;
	}

	.cluster-info-container :global(.cluster-info-header) {
		height: 56px;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
</style>
