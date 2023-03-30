<script lang="ts">
	import { Paper, type SSX, Typography, Box, ListSubHeader } from '@ui';
	import NavLink from './nav-link.svelte';

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
		},
	};
</script>

<Paper {ssx} square backgroundColor="navbar" color="on-navbar">
	<Box
		ssx={{ $self: { display: 'flex', flexDirection: 'column', gap: '4px', padding: '0px 1rem' } }}
	>
		<Typography variant="h5">Cluster</Typography>
		<Typography variant="caption" color="text-secondary-on-dark">cluster name</Typography>
	</Box>
	<Box class="nav-drawer-section">
		<NavLink variant="body1">Packages</NavLink>
	</Box>
	<Box class="nav-drawer-section inset-shadow">
		<Typography color="text-secondary-on-dark">Tools</Typography>
		<NavLink variant="body1">SBOM Comparison</NavLink>
	</Box>
</Paper>
