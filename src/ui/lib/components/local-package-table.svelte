<script lang="ts">
	import { Packages } from '$lib/api';
	import { Paper, Typography, Box, type SSX } from '@ui';
	import Tooltip from './tooltip.svelte';
	import ZarfChip from './zarf-chip.svelte';
	import { page } from '$app/stores';
	import type { APIZarfPackage } from '$lib/api-types';

	const initPkg = $page.url.searchParams.get('init');

	async function readPackages(): Promise<APIZarfPackage[]> {
		const paths = initPkg ? await Packages.findInit() : await Packages.findInHome();
		const packages = paths.map((p) => Packages.read(p));
		return Promise.all(packages);
	}

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			maxHeight: '288px',
			'& .local-package-list-header': {
				height: '56px',
				padding: '16px',
				display: 'flex',
				alignItems: 'center',
				gap: '20px',
				borderBottomLeftRadius: '0px',
				borderBottomRightRadius: '0px',
				'& .tooltip-trigger': {
					display: 'flex',
					alignItems: 'end',
					color: 'var(--action-active-56p)',
				},
			},
			'& .local-package-list-body': {
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
			'& .local-package-list-footer': {
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
					'&.description': {
						minWidth: '240px',
						width: '21.5%',
					},
				},
			},
		},
	};

	const tableLabels = ['name', 'version', 'tags', 'description'];
	$: initString = (initPkg && 'Init') || '';
</script>

<Box {ssx} class="local-package-list-container">
	<Paper class="local-package-list-header" elevation={1}>
		<Typography variant="th">Local Directory</Typography>
		<Tooltip>Something Something Local Directory</Tooltip>
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
	<Paper class="local-package-list-body" square elevation={1}>
		{#await readPackages()}
			<div class="no-packages">
				<Typography color="primary" variant="body1"
					>Searching for local Zarf{initString} Packages</Typography
				>
			</div>
		{:then packages}
			{#if !packages.length}
				<div class="no-packages">
					<Typography color="blue-200" variant="body1"
						>No Zarf{initString} Packages found on local system</Typography
					>
				</div>
			{:else}
				{#each packages as pkg}
					<Paper class="package-table-row" square elevation={1}>
						<Typography variant="body2" class="package-table-td name" element="span">
							<span class="material-symbols-outlined" style="color:var(--success);">
								check_circle
							</span>
							<span>{pkg.zarfPackage.metadata?.name}</span>
						</Typography>

						<Typography variant="body2" class="package-table-td version">
							<ZarfChip>
								{pkg.zarfPackage.metadata?.version}
							</ZarfChip>
						</Typography>

						<Typography variant="body2" class="package-table-td tags">
							<ZarfChip>
								{pkg.zarfPackage?.build?.architecture}
							</ZarfChip>
							<ZarfChip>
								{pkg.zarfPackage.kind}
							</ZarfChip>
						</Typography>
						<Typography variant="body2" class="package-table-td description">
							{pkg.zarfPackage.metadata?.description}
						</Typography>
					</Paper>
				{/each}
			{/if}
		{:catch err}
			<div class="no-packages">
				<Typography color="error" variant="body1">{err.message}</Typography>
			</div>
		{/await}
	</Paper>
	<Paper class="local-package-list-footer" elevation={1} />
</Box>
