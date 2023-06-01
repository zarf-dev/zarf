<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { Packages } from '$lib/api';
	import { page } from '$app/stores';
	import { Paper, Typography, Box, Button, type SSX } from '@ui';
	import { pkgStore } from '$lib/store';
	import Spinner from './spinner.svelte';
	import Tooltip from './tooltip.svelte';
	import type { APIZarfPackage } from '$lib/api-types';
	import ZarfChip from './zarf-chip.svelte';
	import type { EventParams } from '$lib/http';
	import ZarfDialog from './zarf-dialog.svelte';
	import ButtonDense from './button-dense.svelte';
	import { onDestroy } from 'svelte';

	const initPkg = $page.url.searchParams.get('init');
	let packages: APIZarfPackage[] = [];
	let stream: AbortController;
	let noPackagesToggle: () => void;

	async function streamPackages(): Promise<void> {
		return new Promise((resolve, reject) => {
			const eventParams: EventParams = {
				onmessage: (event) => {
					try {
						const pkg = JSON.parse(event.data);
						console.log(pkg);
						packages = [...packages, pkg];
					} catch {
						console.log('here');
						console.log(event.data);
					}
				},
				onerror: (event) => {
					reject(event);
				},
				onclose: () => {
					resolve();
				},
			};
			stream = initPkg
				? Packages.initPackageStream(eventParams)
				: Packages.packageStream(eventParams);
		});
	}

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			flexGrow: '1',
			height: '100%',
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
				minHeight: '100%',
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
				minHeight: '48px',
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

	onDestroy(() => {
		if (stream) {
			stream.abort();
		}
	});

	const tableLabels = ['name', 'version', 'tags', 'description'];
	$: initString = (initPkg && 'Init') || '';
	$: tooltip =
		(initPkg && 'in the execution, current working, and .zarf-cache directories') ||
		'in the current working directory';
	$: console.log(packages);
</script>

<Box {ssx} class="local-package-list-container">
	<Paper class="local-package-list-header" elevation={1}>
		<Typography variant="th">Local Directory</Typography>
		<Tooltip>
			This table shows all of the Zarf{initString} packages that exist {tooltip}.
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
		{#each packages as pkg}
			<Paper class="package-table-row" square elevation={1}>
				<Typography variant="body2" class="package-table-td name" element="span">
					<span class="material-symbols-outlined" style="color:var(--success);">check_circle</span>
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
		{#await streamPackages()}
			<div class="no-packages">
				<Spinner color="blue-200" />
				<Typography color="blue-200" variant="body1">Searching working directory.</Typography>
			</div>
		{:then}
			{#if !packages.length}
				<ZarfDialog bind:toggleDialog={noPackagesToggle} open titleText="No Packages Found">
					<Typography variant="body2" color="text-secondary-on-dark">
						No Zarf packages were found in the current working directory. Would you like to search
						the home directory?
					</Typography>
					<svelte:fragment slot="actions">
						<ButtonDense on:click={() => goto('/')} variant="outlined" backgroundColor="white">
							cancel deployment
						</ButtonDense>
						<ButtonDense on:click={noPackagesToggle} variant="raised" backgroundColor="primary">
							Search Directory
						</ButtonDense>
					</svelte:fragment>
				</ZarfDialog>
			{/if}
		{:catch err}
			<div class="no-packages">
				<Typography color="error" variant="body1">{err.message}</Typography>
			</div>
		{/await}
	</Paper>
	<Paper class="local-package-list-footer" elevation={1} />
</Box>
