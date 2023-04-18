<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Packages } from '$lib/api';
	import { Paper, Typography, Box, Button, type SSX } from '@ui';
	import Tooltip from './tooltip.svelte';
	import ZarfChip from './zarf-chip.svelte';
	import { page } from '$app/stores';
	import type { APIZarfPackage } from '$lib/api-types';
	import { pkgStore } from '$lib/store';
	import { goto } from '$app/navigation';
	import Spinner from './spinner.svelte';

	const initPkg = $page.url.searchParams.get('init');

	async function readPackages(): Promise<APIZarfPackage[]> {
		const paths = initPkg ? await Packages.findInit() : await Packages.findInHome();
		// resolve all reads regardless of success or failure.
		const result = await Promise.allSettled(paths.map((p) => Packages.read(p)));
		// Filter out failed reads
		// TODO: Handle and present packages that could not be read.
		const settledFulfilled = result.filter(
			(p) => p.status === 'fulfilled'
		) as PromiseFulfilledResult<APIZarfPackage>[];

		// Return the values from the fulfilled results.
		return settledFulfilled.map((p) => p.value);
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

				'& .tooltip': {
					wordBreak: 'break-word',
					width: '500px',
				},
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
					gap: '10px',
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
					'&.deploy': {
						display: 'flex',
						flexGrow: '1',
						justifyContent: 'end',
						alignItems: 'center',
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
		<Tooltip>
			This table shows all of the Zarf{initString} packages that exist on your local machine.
		</Tooltip>
	</Paper>
	<Paper class="package-table-head-row package-table-row" square elevation={1}>
		{#each tableLabels as l}
			<Typography
				class="package-table-td {l.split(' ').join('-')}"
				variant="overline"
				color="text-secondary-on-dark"
			>
				{l}
			</Typography>
		{/each}
	</Paper>
	<Paper class="local-package-list-body" square elevation={1}>
		{#await readPackages()}
			<div class="no-packages">
				<Spinner color="blue-200" />
				<Typography color="blue-200" variant="body1">
					Searching your local machine for Zarf{initString} Packages. This may take a minute.
				</Typography>
			</div>
		{:then packages}
			{#if !packages.length}
				<div class="no-packages">
					<Typography color="blue-200" variant="body1">
						No Zarf{initString} Packages found on local system
					</Typography>
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
							{#if !initPkg && pkg.zarfPackage?.metadata?.version}
								<ZarfChip>
									{pkg.zarfPackage.metadata.version}
								</ZarfChip>
							{:else if initPkg && pkg.zarfPackage?.build?.version}
								<ZarfChip>
									{pkg.zarfPackage.build.version}
								</ZarfChip>
							{/if}
						</Typography>

						<Typography variant="body2" class="package-table-td tags">
							{#if pkg.zarfPackage?.build?.architecture}
								<ZarfChip>
									{pkg.zarfPackage.build.architecture}
								</ZarfChip>
							{/if}
							<ZarfChip>
								{pkg.zarfPackage.kind}
							</ZarfChip>
						</Typography>
						<Typography variant="body2" class="package-table-td description">
							{pkg.zarfPackage.metadata?.description}
						</Typography>
						<Box class="package-table-td deploy">
							<Button
								title={pkg.zarfPackage.metadata?.name}
								backgroundColor="on-surface"
								on:click={() => {
									pkgStore.set(pkg);
									goto(`/packages/${pkg.zarfPackage.metadata?.name}/configure`);
								}}
							>
								deploy
							</Button>
						</Box>
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
