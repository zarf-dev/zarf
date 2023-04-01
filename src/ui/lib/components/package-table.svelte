<script lang="ts">
	import { Packages } from '$lib/api';
	import { Paper, Typography, Chip, type SSX } from '@ui';
	import ButtonDense from './button-dense.svelte';
	import ZarfChip from './zarf-chip.svelte';

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			maxHeight: '280px',
			'& .package-list-header': {
				height: '56px',
				padding: '16px',
				display: 'flex',
				alignItems: 'center',
				justifyContent: 'space-between',
			},
			'& .package-list-body': {
				height: '100px',
				boxShadow: '0px -1px 0px 0px rgba(255, 255, 255, 0.12) inset',
				overflowX: 'hidden',
				overflowY: 'scroll',
				'& .no-packages': {
					width: '100%',
					height: '100%',
					display: 'flex',
					justifyContent: 'center',
					alignItems: 'center',
				},
			},
			'& .package-list-footer': {
				height: '48px',
			},
			'& .package-table-head-row': {
				'& .package-table-td.name': {
					paddingLeft: '48px',
				},
			},
			'& .package-table-row': {
				display: 'flex',
				alignItems: 'center',
				boxShadow: 'inset 0px -1px 0px rgba(255,255,255,0.12)',
				'& .package-table-td': {
					padding: '8px 16px',
					'&.name': {
						minWidth: '224px',
						width: '20%',
						display: 'flex',
						alignItems: 'center',
						gap: '10.67px',
						flexWrap: 'wrap',
						wordBreak: 'break-all',
					},
					'&.version': {
						minWidth: '134px',
						width: '12%',
					},
					'&.tags': {
						minWidth: '276px',
						width: '24.8%',
					},
					'&.signed-by': {
						minWidth: '240px',
						width: '21.5%',
					},
				},
			},
		},
	};

	const tableLabels = ['name', 'version', 'tags', 'signed by'];
</script>

<Paper {ssx} class="package-list-container" square>
	<Paper class="package-list-header" square elevation={1}>
		<Typography variant="th">Packages</Typography>
		<ButtonDense backgroundColor="white" variant="outlined">Deploy Package</ButtonDense>
	</Paper>
	<Paper class="package-table-head-row package-table-row" square elevation={1}>
		{#each tableLabels as l}
			<Typography
				class="package-table-td {l.split(' ').join('-')}"
				variant="overline"
				color="text-secondary-on-dark">{l}</Typography
			>
		{/each}
	</Paper>
	<Paper class="package-list-body" square elevation={1}>
		{#await Packages.getDeployedPackages() then packages}
			{#if !packages.length}
				<div class="no-packages">
					<Typography color="primary" variant="body1">No Packages have been Deployed</Typography>
				</div>
			{:else}
				{#each packages as pkg}
					<Paper class="package-table-row" square elevation={1}>
						<Typography variant="body2" class="package-table-td name" element="span">
							<span class="material-symbols-outlined" style="color:var(--success);">
								check_circle
							</span>
							<span>{pkg.name}</span>
						</Typography>

						<Typography variant="body2" class="package-table-td version">
							<ZarfChip>
								{pkg.data?.build?.version}
							</ZarfChip>
						</Typography>
					</Paper>
				{/each}
			{/if}
		{/await}
	</Paper>
	<Paper class="package-list-footer" square elevation={1} />
</Paper>
