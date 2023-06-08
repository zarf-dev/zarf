<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import ZarfLogoDark from '@images/zarf-logo.png';

	import { Paper, type SSX } from '@ui';
	import ThemeToggle from './theme-toggle.svelte';
	const headerSSX: SSX = {
		$dark: {
			'& $self': {
				backgroundImage: 'unset',
			},
		},
		$self: {
			width: '100vw',
			height: '3.5rem',
			display: 'flex',
			justifyContent: 'space-between',
			alignItems: 'center',
			padding: '4px 24px',
			zIndex: '1',
		},
	};
</script>

<Paper
	class="global-nav-bar"
	ssx={headerSSX}
	backgroundColor="global-nav"
	color="on-global-nav"
	elevation={2}
	square
>
	<div class="nav-right">
		<a href="/">
			<img src={ZarfLogoDark} height="48px" width="107px" alt="Zarf logo link to home" />
		</a>
	</div>
	<div class="nav-left"><ThemeToggle /></div>
</Paper>
