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
	import { onMount } from 'svelte';

	const initPkg = $page.url.searchParams.get('init');
	const tableLabels = ['name', 'version', 'tags', 'description'];

	let packageMap: Record<string, APIZarfPackage> = {};
	let stream: AbortController;
	let noPackagesToggle: () => void;
	let doneStreaming: boolean = false;
	let expandedSearch: boolean = false;

	function getEventParams(resolve: () => void, reject: (error: any) => void): EventParams {
		return {
			onmessage: (event) => {
				try {
					const pkg = JSON.parse(event.data) as APIZarfPackage;
					console.log(pkg.path);
					packageMap = { ...packageMap, [pkg.path]: pkg };
				} catch {
					console.log(event.data);
				}
			},
			onerror: (event) => {
				doneStreaming = true;
				reject(event);
			},
			onclose: () => {
				doneStreaming = true;
				resolve();
			},
		};
	}

	async function findPackages(): Promise<void> {
		return new Promise((resolve, reject) => {
			const eventParams = getEventParams(resolve, reject);
			stream = initPkg ? Packages.findInit(eventParams) : Packages.find(eventParams);
		});
	}

	async function findPackagesRecursively(): Promise<void> {
		doneStreaming = false;
		expandedSearch = true;
		return new Promise((resolve, reject) => {
			const isInit = initPkg ? true : false;
			const eventParams = getEventParams(resolve, reject);
			stream = Packages.findHome(eventParams, isInit);
		});
	}

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			flexGrow: '1',
			minHeight: '50vh',
			maxHeight: '75vh',
			marginBottom: '32px',
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
				height: 'calc(100% - 56px - 48px - 48px)',
				boxShadow: '0px -1px 0px 0px rgba(255, 255, 255, 0.12) inset',
				overflowX: 'hidden',
				overflowY: 'scroll',
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
				'& .loading-packages': {
					display: 'flex',
					width: '100%',
					gap: '10px',
					height: '68px',
					justifyContent: 'center',
					alignItems: 'center',
				},
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

	onMount(() => {
		findPackages();
		return () => {
			if (stream) {
				stream.abort();
			}
		};
	});

	$: initString = (initPkg && 'Init') || '';
	$: tooltip =
		(initPkg && 'in the execution, current working, and .zarf-cache directories') ||
		'in the current working directory';
	$: searchText = (expandedSearch && 'Searching home directory.') || 'Searching working directory.';
	$: packages = Object.values(packageMap);
</script>

<Box {ssx} class="local-package-list-container">
	<Paper class="local-package-list-header" elevation={1}>
		<Typography variant="th">Local Directory</Typography>
		<Tooltip>
			This table shows all of the Zarf{initString} packages that exist {tooltip}.
		</Tooltip>
		{#if doneStreaming || expandedSearch}
			<ButtonDense
				variant="outlined"
				backgroundColor="white"
				style="margin-left: auto"
				on:click={findPackagesRecursively}
			>
				Expanded Search
			</ButtonDense>
		{/if}
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
		{#if !doneStreaming}
			<Paper class="package-table-row" square elevation={1}>
				<div class="loading-packages">
					<Spinner color="blue-200" />
					<Typography color="blue-200" variant="body1">{searchText}</Typography>
				</div>
			</Paper>
		{/if}
		{#if doneStreaming && !packages.length && expandedSearch}
			<Paper class="package-table-row" square elevation={1}>
				<div class="loading-packages">
					<span style="color: var(--warning)" class="material-symbols-outlined">warning</span>
					<Typography color="on-surface" variant="body1">No packages found.</Typography>
				</div>
			</Paper>
		{/if}
		{#if !packages.length && doneStreaming && !expandedSearch}
			<ZarfDialog bind:toggleDialog={noPackagesToggle} open titleText="No Packages Found">
				<Typography variant="body2" color="text-secondary-on-dark">
					No Zarf packages were found in the current working directory. Would you like to search the
					home directory?
				</Typography>
				<svelte:fragment slot="actions">
					<ButtonDense on:click={() => goto('/')} variant="outlined" backgroundColor="white">
						cancel deployment
					</ButtonDense>
					<ButtonDense
						on:click={findPackagesRecursively}
						variant="raised"
						backgroundColor="on-surface"
					>
						Search Directory
					</ButtonDense>
				</svelte:fragment>
			</ZarfDialog>
		{/if}
	</Paper>
	<Paper class="local-package-list-footer" elevation={1} />
</Box>
