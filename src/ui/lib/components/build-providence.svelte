<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Box, Typography, type SSX, currentTheme } from '@ui';
	import type { ZarfBuildData } from '$lib/api-types';
	import { pkgSbomStore, pkgStore } from '$lib/store';
	import { Packages } from '$lib/api';
	import CopyToClipboard from './copy-to-clipboard.svelte';
	import PackageSectionHeader from './package-section-header.svelte';
	import ButtonDense from './button-dense.svelte';

	export let build: ZarfBuildData | undefined;

	// Will be bound from the CopyToClipboard component.
	let copyToClipboard: () => void;
	const labels = ['terminal', 'user', 'architecture', 'timestamp', 'version'];

	function getLabelValue(label: string) {
		return build ? Object(build)[label] ?? '' : '';
	}

	async function getSbom() {
		let sbom = $pkgSbomStore;
		if (!$pkgSbomStore) {
			sbom = await Packages.sbom($pkgStore.path);
			pkgSbomStore.set(sbom);
		}
		return sbom;
	}

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			gap: '8px',
			padding: '0px 32px',

			'& .build-data': {
				display: 'flex',
				gap: '11px',
				'& .build-list': {
					display: 'flex',
					flexDirection: 'column',
				},
			},
			'& #sbom-info': {
				marginBottom: '8px',
			},
		},
	};

	$: sbomUrl = ($pkgSbomStore && `/sbom-viewer/${$pkgSbomStore.path.split('/').pop()}`) || '';
</script>

<PackageSectionHeader icon="secured_layer">
	Supply Chain
	<span slot="tooltip">
		Supply chain information is used to help determine if a package can be trusted. It includes
		declarative data regarding how this package was built. Build providence includes metadata about
		the build and where the package was created. SBOM includes information on all of the code,
		images, and resources contained in this package.
	</span>
	<svelte:fragment slot="actions">
		{#if sbomUrl}
			<ButtonDense variant="outlined" href={sbomUrl} target="_blank" backgroundColor="white">
				view sbom
			</ButtonDense>
		{/if}
	</svelte:fragment>
</PackageSectionHeader>
<Box {ssx}>
	<Typography variant="subtitle2">Build Providence</Typography>
	<div class="build-data">
		<div class="build-list">
			{#each labels as label}
				<Typography
					variant="caption"
					color="text-secondary-on-${$currentTheme}"
					style="text-transform: capitalize;"
				>
					{label}:
				</Typography>
			{/each}
		</div>
		<div class="build-list">
			{#each labels as label}
				<Typography variant="caption">{getLabelValue(label)}</Typography>
			{/each}
		</div>
	</div>
	<Typography variant="subtitle2">Software Bill of Materials (SBOM)</Typography>
	{#await getSbom() then sbom}
		<Typography
			id="sbom-info"
			variant="body2"
			color="text-secondary-on-dark"
			ssx={{
				$self: {
					display: 'flex',
					alignItems: 'center',
					gap: '8px',
					marginTop: '8px',
				},
			}}
		>
			<span style="color: var(--warning)" class="material-symbols-outlined">warning</span>
			<Typography variant="inherit" element="span">
				{sbom?.sboms.length} artifacts to be reviewed. Review the package SBOM files before continuing.
				The SBOM files are in a temporary folder that will be removed upon deployment.
			</Typography>
		</Typography>
		<Typography element="p" variant="body2" color="text-secondary-on-dark">
			To view the SBOM files, open this link:
			<Typography
				href={`/sbom-viewer/${sbom?.path.split('/').pop()}`}
				target="_blank"
				color="primary"
				style="text-decoration: underline; cursor: pointer;"
				variant="inherit"
				element="a"
			>
				{sbom?.path}
			</Typography>
			<CopyToClipboard bind:copyToClipboard text={sbom?.path || ''} variant="h6" element="span" />
		</Typography>
	{:catch error}
		<Typography variant="body2">{error}</Typography>
	{/await}
</Box>
