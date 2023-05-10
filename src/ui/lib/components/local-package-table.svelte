<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Packages } from '$lib/api';
	import {
		Paper,
		Typography,
		Box,
		Button,
		type SSX,
		Dialog,
		ListItem,
		List,
		ListItemAdornment,
	} from '@ui';
	import Tooltip from './tooltip.svelte';
	import ZarfChip from './zarf-chip.svelte';
	import { page } from '$app/stores';
	import type { APIExplorerFile, APIZarfPackage } from '$lib/api-types';
	import { pkgStore } from '$lib/store';
	import { goto } from '$app/navigation';
	import Spinner from './spinner.svelte';
	import ButtonDense from './button-dense.svelte';
	import ZarfDialog from './zarf-dialog.svelte';
	import Divider from './divider.svelte';

	const initPkg = $page.url.searchParams.get('init');

	let exploreErrorToggle: () => void;
	let explorerToggle: () => void;
	let selectedPackage: string;
	let currentDir: string;
	let homeDir: string;
	let selectedPath: string;

	function backOneFolder() {
		const splitPath = currentDir.split('/');
		splitPath.pop();
		currentDir = splitPath.join('/');
	}

	async function readPackages(): Promise<APIZarfPackage[]> {
		const paths = initPkg ? await Packages.findInit() : await Packages.find();
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

	async function explorePath(path?: string): Promise<APIExplorerFile[]> {
		const explorer = initPkg ? await Packages.exploreInit(path) : await Packages.explore(path);
		if (!path) {
			homeDir = explorer.dir;
		}
		currentDir = explorer.dir;
		selectedPath = '';

		return explorer.files;
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

	const tableLabels = ['name', 'version', 'tags', 'description'];
	$: initString = (initPkg && 'Init') || '';
	$: tooltip =
		(initPkg && 'in the execution, current working, and .zarf-cache directories') ||
		'in the current working directory';
</script>

<Box {ssx} class="local-package-list-container">
	<Paper class="local-package-list-header" elevation={1}>
		<Typography variant="th">Local Directory</Typography>
		<Tooltip>
			This table shows all of the Zarf{initString} packages that exist {tooltip}.
		</Tooltip>
		<ButtonDense variant="outlined" backgroundColor="white" on:click={explorerToggle}>
			Select Package
		</ButtonDense>
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
<ZarfDialog
	bind:toggleDialog={exploreErrorToggle}
	happyZarf={false}
	titleText="Failed to load Zarf{initString} package."
>
	<Typography variant="body1" color="text-secondary-on-dark">
		Failed to open file {selectedPackage}
	</Typography>
	<ButtonDense
		slot="actions"
		variant="outlined"
		backgroundColor="white"
		on:click={exploreErrorToggle}
	>
		Ok
	</ButtonDense>
</ZarfDialog>
<ZarfDialog bind:toggleDialog={explorerToggle} titleText="Select ZarfPackage">
	<svelte:fragment>
		<Typography variant="body1" color="text-secondary-on-dark">
			{currentDir}
		</Typography>
		<List
			ssx={{
				$self: {
					maxHeight: '150px',
					overflowY: 'scroll',
					padding: '0px',
					'& > li': {
						display: 'flex',
						alignItems: 'center',
						padding: '0px',
					},
					'& .list-item-adornment': { color: 'var(--text-secondary-on-dark)' },
					'& .divider': { height: '1px', boxShadow: 'inset 0px -1px 0px rgba(255,255,255,0.12)' },
				},
			}}
		>
			{#await explorePath(currentDir)}
				<Box
					ssx={{
						$self: {
							display: 'flex',
							alignItems: 'center',
							justifyContent: 'center',
						},
					}}
				>
					<Spinner color="blue-200" />
				</Box>
			{:then files}
				{#each files as file}
					<ListItem
						title={file.path}
						class="list-item"
						selected={selectedPath === file.path}
						on:click={async () => {
							selectedPath = file.path;
							if (file.isDir) {
								currentDir = file.path;
							} else {
								try {
									const pkg = await Packages.read(file.path);
									pkgStore.set(pkg);
									goto(`/packages/${pkg.zarfPackage.metadata?.name}/configure`);
								} catch (err) {
									selectedPackage = file.path;
									explorerToggle();
									exploreErrorToggle();
								}
							}
						}}
						text={file.path.split('/').pop() || ''}
					>
						<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined">
							{file.isDir ? 'folder' : 'description'}
						</ListItemAdornment>
					</ListItem>
					<div class="divider" />
				{/each}
			{:catch err}
				<Typography variant="body1" color="error">
					{err.message}
				</Typography>
			{/await}
		</List>
	</svelte:fragment>
	<svelte:fragment slot="actions">
		{#if currentDir !== homeDir}
			<ButtonDense variant="raised" backgroundColor={'white'} on:click={backOneFolder}>
				Back
			</ButtonDense>
		{/if}
		<ButtonDense variant="outlined" backgroundColor="white" on:click={explorerToggle}>
			Close
		</ButtonDense>
	</svelte:fragment>
</ZarfDialog>
