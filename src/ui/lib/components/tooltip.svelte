<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import Icon from './icon.svelte';
	import { computePosition, flip, shift, offset, arrow, autoUpdate } from '@floating-ui/dom';
	import { onDestroy, onMount } from 'svelte';
	let tooltip: HTMLElement;
	let icon: HTMLElement;
	let arrowRef: HTMLElement;
	type Placement =
		| 'top'
		| 'bottom'
		| 'left'
		| 'right'
		| 'top-start'
		| 'top-end'
		| 'bottom-start'
		| 'bottom-end'
		| 'left-start'
		| 'left-end'
		| 'right-start'
		| 'right-end';
	export let placement: Placement = 'top';

	function update() {
		computePosition(icon, tooltip, {
			placement,
			middleware: [offset(6), flip(), shift(), arrow({ element: arrowRef })],
		}).then(({ x, y, placement, middlewareData }) => {
			Object.assign(tooltip.style, {
				left: `${x}px`,
				top: `${y}px`,
			});

			const arrowX = middlewareData.arrow?.x;
			const arrowY = middlewareData.arrow?.y;

			const staticSide: any = {
				top: 'bottom',
				right: 'left',
				bottom: 'top',
				left: 'right',
			}[placement.split('-')[0]];

			Object.assign(arrowRef.style, {
				left: arrowX != undefined ? `${arrowX}px` : '',
				top: arrowY != undefined ? `${arrowY}px` : '',
				right: '',
				bottom: '',
				[staticSide]: '-4px',
			});
		});
	}

	function showTooltip() {
		tooltip.style.display = 'block';
		update();
	}

	function hideTooltip() {
		tooltip.style.display = '';
	}

	let cleanup: () => void;

	onMount(() => {
		cleanup = autoUpdate(icon, tooltip, update);
	});

	onDestroy(() => {
		cleanup();
	});
</script>

<button
	aria-describedby="tooltip"
	bind:this={icon}
	on:mouseenter={showTooltip}
	on:mouseleave={hideTooltip}
	on:focus={showTooltip}
	on:blur={hideTooltip}
	class="tooltip-trigger"
>
	<Icon variant="info" /></button
>
<div role="tooltip" bind:this={tooltip} class="tooltip">
	<slot />
	<div bind:this={arrowRef} class="arrow" />
</div>

<style>
	.tooltip {
		display: none;
		width: min-content;
		word-break: break;
		position: absolute;
		top: 0;
		left: 0;
		background: var(--mdc-theme-surface);
		border: 1px solid var(--mdc-theme-on-surface);
		font-weight: bold;
		padding: 5px;
		border-radius: 4px;
		font-size: 90%;
	}
	.arrow {
		position: absolute;
		background: var(--mdc-theme-surface);
		border-bottom: 1px solid var(--mdc-theme-on-surface);
		border-right: 1px solid var(--mdc-theme-on-surface);
		width: 8px;
		height: 8px;
		transform: rotate(45deg);
	}
	.tooltip-trigger {
		background: inherit;
		border: none;
		padding: 0;
	}
</style>
