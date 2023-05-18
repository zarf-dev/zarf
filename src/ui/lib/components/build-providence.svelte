<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Box, Typography, type SSX, currentTheme } from '@ui';
	import type { ZarfBuildData } from '$lib/api-types';
	import { pkgStore } from '$lib/store';
	import { Packages } from '$lib/api';
	import CopyToClipboard from './copy-to-clipboard.svelte';

	export let build: ZarfBuildData | undefined;

	let copyToClipboard: () => void;
	const labels = ['terminal', 'user', 'architecture', 'timestamp', 'version'];

	function getLabelValue(label: string) {
		return build ? Object(build)[label] ?? '' : '';
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
		},
	};
</script>

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
	{#await Packages.sbom($pkgStore.path) then sbom}
		<Typography element="p" variant="body2" color="text-secondary-on-dark">
			This package has {sbom.sboms.length} images with software SBOMs included. You can view them now
			in the zarf-sbom folder in this directory or to go directly to one, open this in your browser:
			<Typography
				color="primary"
				style="text-decoration: underline; cursor: pointer;"
				variant="inherit"
				element="span"
				on:click={copyToClipboard}
			>
				{sbom.path}
			</Typography>
			<CopyToClipboard bind:copyToClipboard text={sbom.path} variant="h6" />
		</Typography>
	{:catch error}
		<Typography variant="body2">{error}</Typography>
	{/await}
</Box>
