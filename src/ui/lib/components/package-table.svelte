<script lang="ts">
	import { Paper, Typography, Box, type SSX } from '@ui';
	import ButtonDense from './button-dense.svelte';
	import ZarfChip from './zarf-chip.svelte';
	import { deployedPkgStore } from '$lib/store';

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
				borderBottomLeftRadius: '0px',
				borderBottomRightRadius: '0px',
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
				borderTopLeftRadius: '0px',
				borderTopRightRadius: '0px',
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
						display: 'flex',
						flexWrap: 'wrap',
						gap: '4px',
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

<Box {ssx} class="package-list-container">
	<Paper class="package-list-header" elevation={1}>
		<Typography variant="th">Packages</Typography>
		<ButtonDense backgroundColor="white" variant="outlined" href="/packages">
			Deploy Package
		</ButtonDense>
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
		{#if !$deployedPkgStore}
			<div class="no-packages">
				<Typography color="primary" variant="body1">Searching for Deployed Packages</Typography>
			</div>
		{:else if $deployedPkgStore.err}
			<div class="no-packages">
				<Typography color="primary" variant="body1">{$deployedPkgStore.err.message}</Typography>
			</div>
		{:else if !$deployedPkgStore.pkgs || !$deployedPkgStore.pkgs.length}
			<div class="no-packages">
				<Typography color="blue-200" variant="body1">No Packages have been Deployed</Typography>
			</div>
		{:else}
			{#each $deployedPkgStore.pkgs as pkg}
				<Paper class="package-table-row" square elevation={1}>
					<Typography variant="body2" class="package-table-td name" element="span">
						<span class="material-symbols-outlined" style="color:var(--success);">
							check_circle
						</span>
						<span>{pkg.name}</span>
					</Typography>

					<Typography variant="body2" class="package-table-td version">
						{#if pkg.data.metadata?.version}
							<ZarfChip>
								{pkg.data.metadata?.version}
							</ZarfChip>
						{/if}
					</Typography>

					<Typography variant="body2" class="package-table-td tags">
						<ZarfChip>
							{pkg.data?.build?.architecture}
						</ZarfChip>
						<ZarfChip>
							{pkg.data?.kind}
						</ZarfChip>
					</Typography>
				</Paper>
			{/each}
		{/if}
	</Paper>
	<Paper class="package-list-footer" elevation={1} />
</Box>
