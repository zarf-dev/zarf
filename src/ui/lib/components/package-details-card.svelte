<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import type { ZarfPackage } from '$lib/api-types';
	import { Typography, type SSX, Box } from '@ui';
	import ZarfChip from './zarf-chip.svelte';
	export let pkg: ZarfPackage;

	const ssx: SSX = {
		$self: {
			padding: '0px 52px',
			display: 'flex',
			flexDirection: 'column',
			gap: '18px',
			'& .row': {
				display: 'flex',
				gap: '12px',
				'& > .h5': {
					marginRight: '15px',
				},
			},
			'& .description': {
				color: 'var(--text-secondary-on-dark)',
			},
		},
		$light: {
			'& $self': {
				'& .description': {
					color: 'var(--text-secondary-on-light)',
				},
				'& .zarf-chip': {
					backgroundColor: 'var(--grey-300)',
					color: 'var(--text-secondary-on-light)',
				},
			},
		},
	};
</script>

<Box {ssx} class="package-details">
	<div class="row">
		<Typography variant="h5">{pkg.metadata?.name}</Typography>
		{#if pkg.metadata?.version}
			<ZarfChip>{pkg.metadata.version}</ZarfChip>
		{/if}
		<ZarfChip>{pkg.metadata?.architecture}</ZarfChip>
		<ZarfChip>{pkg.kind}</ZarfChip>
	</div>
	<Typography variant="body2" class="description">
		{pkg.metadata?.description}
	</Typography>
</Box>
