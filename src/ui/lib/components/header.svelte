<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script>
	import { clusterStore } from '$lib/store';
	import logo from '@images/zarf-logo.png';
	import Icon from './icon.svelte';
	import ThemeToggle from './theme-toggle.svelte';
</script>

<header class="header mdc-elevation--z8">
	<div class="header-start">
		<a href="/"><img alt="Zarf logo" id="logo" src={logo} width="100" /></a>
		<div id="cluster-selector">
			{#if $clusterStore}
				> {$clusterStore.distro}
			{/if}
		</div>
	</div>

	<div class="header-end">
		<ThemeToggle />
		<Icon variant="cloud_off" />
	</div>
</header>

<style lang="scss">
	@use '@material/elevation';
	@include elevation.core-styles;

	#logo {
		display: block;
		background-repeat: no-repeat;
		background-size: 100% 100%;
	}
	.header {
		display: flex;
		align-items: center;
		background-color: #0d133d;
		color: var(--mdc-theme-text-secondary-on-dark);
		padding: 0.5rem 1rem;
		justify-content: space-between;
	}
	.header-start {
		display: flex;
		align-items: center;
		gap: 1rem;
	}
	.header-end {
		display: flex;
		justify-content: flex-end;
		gap: 1rem;
		margin-right: 0.5rem;
		align-items: center;
	}
</style>
