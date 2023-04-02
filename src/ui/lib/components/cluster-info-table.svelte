<script lang="ts">
	import { Paper, Typography, Box, type SSX } from '@ui';
	import { clusterStore } from '$lib/store';

	const clusterInfoTableSSX: SSX = {
		$self: {
			height: '160px',
			padding: '16px 24px',
			display: 'flex',
			gap: '38px',
			'& > .cluster-info-table-column': {
				display: 'flex',
				flexDirection: 'column',
				height: '128px',
				width: '166px',
				gap: '18px',
				padding: 'none',
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
		},
	};
</script>

<Box class="cluster-info-table" ssx={clusterInfoTableSSX}>
	<div class="cluster-info-table-column">
		<Typography variant="overline">name</Typography>
		<Typography variant="caption" style="display:flex;">
			<span
				class="material-symbols-outlined"
				style="color:var(--success);line-height:inherit;font-size:24px;"
			>
				check_circle
			</span>
			<Typography variant="inherit" element="span">
				&nbsp;{$clusterStore.host}
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
				<Typography variant="inherit">K7s Rev:</Typography>
			</Typography>
			<Typography element="div" variant="caption" class="label-values">
				<Typography variant="inherit">Ready</Typography>
				<Typography variant="inherit">kind-kind</Typography>
				<Typography variant="inherit">v1.24.0</Typography>
			</Typography>
		</Box>
	</div>
	<div class="cluster-info-table-divider" />
	<div class="cluster-info-table-column">
		<Typography variant="overline">resources</Typography>
	</div>
	<div class="cluster-info-table-divider" />
	<div class="cluster-info-table-column">
		<Typography variant="overline">metadata</Typography>
	</div>
</Box>
