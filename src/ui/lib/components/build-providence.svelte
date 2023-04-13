<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Box, Typography, type SSX, currentTheme } from '@ui';
	import type { ZarfBuildData } from '$lib/api-types';

	export let build: ZarfBuildData | undefined;
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
					style="text-transform: capitalize;">{label}:</Typography
				>
			{/each}
		</div>
		<div class="build-list">
			{#each labels as label}
				<Typography variant="caption">{getLabelValue(label)}</Typography>
			{/each}
		</div>
	</div>
</Box>
