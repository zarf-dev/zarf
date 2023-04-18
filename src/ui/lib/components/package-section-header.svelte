<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import Icon from './icon.svelte';
	import Tooltip from './tooltip.svelte';
	import type { IconVariant } from './icon.svelte';
	import { Paper, type SSX, Typography } from '@ui';
	export let icon: IconVariant = 'package';

	const ssx: SSX = {
		$self: {
			padding: '0px 32px',
			display: 'flex',
			height: '56px',
			justifyContent: 'space-between',
			alignItems: 'center',
			stroke: 'var(--on-surface)',
			color: 'var(--on-surface)',
			'& .pkg-section-header-title': {
				display: 'flex',
				gap: '13.5px',
				height: '43px',
				alignItems: 'center',
				'& .tooltip-container': {
					display: 'flex',
					'& .tooltip-trigger': {
						color: 'var(--text-secondary-on-dark)',
					},
					'& .tooltip': {
						wordBreak: 'break-word',
						width: '500px',
					},
				},
			},
		},
	};
</script>

<Paper {ssx} class="pkg-section-header" elevation={1}>
	<div class="pkg-section-header-title">
		<Icon variant={icon} />
		<Typography variant="h5">
			<slot />
		</Typography>
		{#if $$slots.tooltip}
			<div class="tooltip-container"><Tooltip><slot name="tooltip" /></Tooltip></div>
		{/if}
	</div>
	{#if $$slots.actions}
		<div class="pkg-section-header-actions">
			<slot name="actions" />
		</div>
	{/if}
</Paper>
