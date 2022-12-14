<script lang="ts">
	import Icon from './icon.svelte';
	import Tooltip from './tooltip.svelte';
	import type { IconVariant } from './icon.svelte';
	export let icon: IconVariant = 'package-light';
</script>

<div class="pkg-section-header">
	<div class="pkg-section-header-title">
		<Icon variant={icon} />
		<slot name="title" />
		{#if $$slots.tooltip}
			<Tooltip><slot name="tooltip" /></Tooltip>
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
		background-color: var(--mdc-theme-paper-dark);
		color: var(--mdc-theme-on-secondary);
		padding: 0.75rem 2rem;
	}
	.pkg-section-header-actions {
		display: flex;
		align-items: center;
	}
</style>
