<script>
	import { fade } from 'svelte/transition';
	import Hero from './hero.svelte';
	import { onMount } from 'svelte';
	import { Typography } from '@ui';

	export let msg = 'Loading...';

	// Need this to force-enable first onload animation to avoid ugly flash on very fast REST calls
	let ready = false;
	onMount(() => {
		ready = true;
	});
</script>

<Hero>
	{#if ready}
		<div class="spinner-wrapper" in:fade={{ duration: 1000 }}>
			<span class="spinner" />
		</div>
		<Typography variant="h5">Checking for Cluster</Typography>
		<Typography variant="body2" element="span" class="label">{msg}</Typography>
	{/if}
</Hero>

<style lang="scss">
	:global(.spinner-wrapper) {
		background-image: url('@images/zarf-bubbles-right.png');
		display: flex;
		flex-direction: column;
		background-position: center;
		align-items: center;
		margin-left: auto;
		margin-right: auto;
		background-size: 59%;
	}

	:global(.label) {
		text-align: center;
		width: 50%;
	}

	.spinner {
		width: 25rem;
		height: 25rem;
		border-radius: 50%;
		border-top: 2rem solid #7bd5f5;
		border-left: 2rem solid #7bd5f5;
		border-bottom: 2rem solid #7bd5f5;
		border-right: 2rem solid transparent;
		animation: spinner 1s linear infinite;
	}

	@keyframes spinner {
		to {
			transform: rotate(360deg);
		}
	}
</style>
