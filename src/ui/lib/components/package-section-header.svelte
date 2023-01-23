<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import Icon from './icon.svelte';
	import Tooltip from './tooltip.svelte';
	import type { IconVariant } from './icon.svelte';
	export let icon: IconVariant = 'package';
</script>

<div class="pkg-section-header">
	<div class="pkg-section-header-title">
		<Icon variant={icon} />
		<slot name="title" />
		{#if $$slots.tooltip}
			<div class="tooltip-container"><Tooltip><slot name="tooltip" /></Tooltip></div>
		{/if}
	</div>
	{#if $$slots.actions}
		<div class="pkg-section-header-actions">
			<slot name="actions" />
		</div>
	{/if}
</div>

<style>
	.pkg-section-header-title {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	.pkg-section-header {
		display: flex;
		justify-content: space-between;
		background-color: var(--mdc-theme-paper);
		color: var(--mdc-theme-on-secondary);
		stroke: var(--mdc-theme-on-secondary);
		padding: 0.75rem 2rem;
	}
	:global(:root[data-theme='dark']) .pkg-section-header {
		background-color: var(--mdc-theme-surface);
	}
	.pkg-section-header-actions {
		display: flex;
		align-items: center;
	}
	.tooltip-container {
		display: inline-flex;
		align-items: center;
	}
	:global(.tooltip-container .tooltip-trigger) {
		color: rgba(255, 255, 255, 0.56);
	}
</style>
