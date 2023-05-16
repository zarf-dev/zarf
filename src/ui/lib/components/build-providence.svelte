<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Box, Typography, type SSX, currentTheme } from '@ui';
	import type { ZarfBuildData } from '$lib/api-types';
	import { pkgStore } from '$lib/store';
	import { Packages } from '$lib/api';
	import Spinner from './spinner.svelte';
	import ButtonDense from './button-dense.svelte';

	export let build: ZarfBuildData | undefined;
	const labels = ['terminal', 'user', 'architecture', 'timestamp', 'version'];

	function getLabelValue(label: string) {
		return build ? Object(build)[label] ?? '' : '';
	}

	async function launchSbom() {
		try {
			Packages.launchSbom($pkgStore.zarfPackage.metadata!.name!);
		} catch (error) {
			console.error(error);
		}
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
	<Typography variant="subtitle2">Sofware Bill of Materials (SBOM)</Typography>
	{#await Packages.sbom($pkgStore)}
		<Spinner />
	{:then sbom}
		<Typography variant="body2">
			This package has {sbom.sboms.length} images with software SBOMs included. You can view them now
			by clicking the button below. (Note: this will launch in default browser)
		</Typography>
		<ButtonDense variant="outlined" backgroundColor="white" on:click={launchSbom}>
			Launch SBOM
		</ButtonDense>
	{:catch error}
		<Typography variant="body2">{error}</Typography>
	{/await}
</Box>
