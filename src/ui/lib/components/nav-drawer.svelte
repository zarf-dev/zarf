<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Paper, type SSX, Typography, Box } from '@ui';
	import NavLink from './nav-link.svelte';
	import { clusterStore } from '$lib/store';
	import { page } from '$app/stores';

	const ssx: SSX = {
		$self: {
			width: '16rem',
			height: '100%',
			paddingTop: '2.5rem',
			paddingBottom: '1.25rem',
			overflowX: 'hidden',
			overflowY: 'auto',
			display: 'flex',
			flexDirection: 'column',
			alignItems: 'flex-start',
			gap: '30px',
			'& .nav-drawer-section': {
				width: '100%',
				'& > *': {
					padding: '0.75rem 1rem',
				},
			},
			'& .inset-shadow': {
				boxShadow: 'inset 0px -1px 0px rgba(255, 255, 255, 0.12)',
			},
			'& .nav-drawer-header': {
				display: 'flex',
				flexDirection: 'column',
				gap: '4px',
				padding: '0px 1rem',
			},
		},
	};

	$: visible = $page.url.href.includes('packages');
	$: style = (visible && 'display:none;') || '';
</script>

<Paper {ssx} square backgroundColor="global-nav" color="on-global-nav" {style}>
	<Box class="nav-drawer-header">
		<Typography variant="h5">Cluster</Typography>
		{#if $clusterStore?.hasZarf && $clusterStore?.rawConfig}
			<Typography variant="caption" color="text-secondary-on-dark">
				{$clusterStore.rawConfig['current-context']}
			</Typography>
		{:else}
			<Typography
				variant="caption"
				color="text-secondary-on-dark"
				style="display: flex;align-items:center;"
				class="drawer-cluster-not-found"
			>
				<span class="material-symbols-outlined" style="color:var(--warning); font-size:20px;">
					warning
				</span>
				<span>&nbsp;Cluster not connected </span>
			</Typography>
		{/if}
	</Box>
	<Box class="nav-drawer-section">
		<NavLink variant="body1" href="/" selected={$page.route.id === '/'}>Packages</NavLink>
	</Box>
</Paper>
