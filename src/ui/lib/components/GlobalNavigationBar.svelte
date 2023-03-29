<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import ZarfLogoDark from '../../images/png/global-navigation-zarf-logo.png';

	import { Paper, type SSX } from '@ui';
	import ThemeToggle from './theme-toggle.svelte';
	const headerSSX: SSX = {
		$dark: {
			'& $self': {
				backgroundImage: 'unset'
			}
		},
		$self: {
			width: '100vw',
			height: '3.5rem',
			display: 'flex',
			justifyContent: 'space-between',
			alignItems: 'center',
			padding: '4px 24px'
		}
	};
</script>

<Paper ssx={headerSSX} backgroundColor="navbar" color="on-navbar" elevation={2} square>
	<div class="nav-right">
		<a href="/"><img src={ZarfLogoDark} alt="Zarf logo link to home" /></a>
	</div>
	<div class="nav-left"><ThemeToggle /></div>
</Paper>
