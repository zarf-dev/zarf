<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { createEventDispatcher } from 'svelte';

	export let open = false;
	export let duration = 0.2;
	export let placement = 'left';
	export let size: null | string = null;

	let mounted = false;
	const dispatch = createEventDispatcher();

	$: style = `--duration: ${duration}s; --size: ${size};`;

	function scrollLock(open: boolean) {
		if (mounted) {
			document.body.style.overflow = open ? 'hidden' : 'auto';
		}
	}

	$: scrollLock(open);

	function handleClickAway() {
		dispatch('clickAway');
	}

	function handleEscape(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			handleClickAway();
		}
	}

	onMount(() => {
		mounted = true;
		scrollLock(open);
	});
</script>

<aside class="drawer" class:open {style}>
	<!-- @Noxsios TODO: handle drawer on smaller screens (show a close button in upper right) -->
	<div class="overlay" on:click={handleClickAway} on:keydown={handleEscape} />

	<div class="panel {placement}" class:size on:keydown={handleEscape}>
		<slot />
	</div>
</aside>

<style>
	.drawer {
		position: fixed;
		top: 0;
		left: 0;
		height: 100%;
		width: 100%;
		z-index: -1;
		transition: z-index var(--duration) step-end;
	}

	.drawer.open {
		z-index: 99;
		transition: z-index var(--duration) step-start;
	}

	.overlay {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: rgba(100, 100, 100, 0.5);
		opacity: 0;
		z-index: 2;
		transition: opacity var(--duration) ease;
	}

	.drawer.open .overlay {
		opacity: 1;
	}

	.panel {
		position: fixed;
		width: 100%;
		height: 100%;
		background: white;
		z-index: 3;
		transition: transform var(--duration) ease;
		overflow: auto;
	}

	.panel.left {
		left: 0;
		transform: translate(-100%, 0);
	}

	.panel.right {
		right: 0;
		transform: translate(100%, 0);
	}

	.panel.top {
		top: 0;
		transform: translate(0, -100%);
	}

	.panel.bottom {
		bottom: 0;
		transform: translate(0, 100%);
	}

	.panel.left.size,
	.panel.right.size {
		max-width: var(--size);
	}

	.panel.top.size,
	.panel.bottom.size {
		max-height: var(--size);
	}

	.drawer.open .panel {
		transform: translate(0, 0);
	}
</style>
