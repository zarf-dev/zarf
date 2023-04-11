<script lang="ts">
	import { Typography, Box, type SSX } from '@ui';
	import { deployedPkgStore, clusterStore } from '$lib/store';

	let numPackages = 0;

	const ssx: SSX = {
		$self: {
			height: '160px',
			padding: '16px 24px',
			display: 'flex',
			gap: '38px',
			'& .cluster-name': {
				flex: '1',
				wordBreak: 'break-all',
				textOverflow: 'ellipsis',
			},
			'& > .cluster-info-table-column': {
				display: 'flex',
				flexDirection: 'column',
				height: '128px',
				width: '166px',
				gap: '18px',
				padding: 'unset',
				'& > .overline': {
					color: 'var(--text-secondary-on-dark)',
				},
			},
			'& > .cluster-info-table-divider': {
				height: '100px',
				width: '1px',
				border: '1px solid rgba(255, 255, 255, 0.12)',
				alignSelf: 'center',
			},
			'& .label-values-container': {
				display: 'flex',
				justifyContent: 'space-between',
				'& .label-values': {
					display: 'flex',
					flexDirection: 'column',
					'&:first-child': {
						color: 'var(--text-secondary-on-dark)',
					},
				},
			},
			'& .metadata-values': {
				display: 'flex',
				flexDirection: 'column',
				justifyContent: 'center',
				alignItems: 'center',
			},
		},
	};

	$: {
		if (!$deployedPkgStore?.pkgs) {
			numPackages = 0;
		} else {
			numPackages = $deployedPkgStore.pkgs?.length;
		}
	}
	$: currentClusterName = $clusterStore?.rawConfig['current-context'];
	$: currentCluster =
		(currentClusterName && $clusterStore?.rawConfig.contexts[currentClusterName]) || undefined;
</script>

<Box class="cluster-info-table" {ssx}>
	<div class="cluster-info-table-column">
		<Typography variant="overline">name</Typography>
		<Typography variant="caption" style="display:flex;flex: 1;overflow-y:scroll;">
			<span
				class="material-symbols-outlined"
				style="color:var(--success);line-height:inherit;font-size:24px;"
			>
				check_circle
			</span>
			<Typography class="cluster-name" variant="inherit" element="span">
				&nbsp;{currentClusterName}
			</Typography>
		</Typography>
	</div>
	<div class="cluster-info-table-divider" />
	<div class="cluster-info-table-column">
		<Typography variant="overline">details</Typography>
		<Box class="label-values-container">
			<Typography element="div" variant="caption" class="label-values">
				<Typography variant="inherit">Health:</Typography>
				<Typography variant="inherit">User:</Typography>
				<Typography variant="inherit">K8s Rev:</Typography>
			</Typography>
			<Typography element="div" variant="caption" class="label-values" style="font-weight:500;">
				<Typography variant="inherit">Ready</Typography>
				<Typography variant="inherit">{currentCluster?.user || ''}</Typography>
				<Typography variant="inherit">{$clusterStore?.k8sRevision}</Typography>
			</Typography>
		</Box>
	</div>
	<div class="cluster-info-table-divider" />
	<div class="cluster-info-table-column">
		<Typography variant="overline">resources</Typography>
		<Box class="label-values-container">
			<Typography element="div" variant="caption" class="label-values">
				<Typography variant="inherit">MEM:</Typography>
				<Typography variant="inherit">CPU:</Typography>
			</Typography>
			<Typography element="div" variant="caption" class="label-values" style="font-weight:500;">
				<Typography variant="inherit">n/a</Typography>
				<Typography variant="inherit">n/a</Typography>
			</Typography>
		</Box>
	</div>
	<div class="cluster-info-table-divider" />
	<div class="cluster-info-table-column">
		<Typography variant="overline">metadata</Typography>
		<div class="metadata-values">
			<Typography variant="h4">{numPackages}</Typography>
			<Typography variant="caption" color="text-secondary-on-dark">Packages</Typography>
		</div>
	</div>
</Box>
