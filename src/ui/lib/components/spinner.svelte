<script>
	import { fade } from 'svelte/transition';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import Hero from './hero.svelte';
	import { onMount } from 'svelte';

	export let msg = 'Loading...';

	// Need this to force-enable first onload animation to avoid ugly flash on very fast REST calls
	let ready = false;
	onMount(() => {
		ready = true;
	});
</script>

<Hero>
	{#if ready}
		<div
			class="spinner-wrapper"
			style="background-image: url('{bigZarf}')"
			in:fade={{ duration: 1000 }}
		>
			<span class="spinner" />
			<span class="label">{msg}</span>
		</div>
	{/if}
</Hero>

<style lang="scss">
	.spinner-wrapper {
		position: absolute;
		display: flex;
		background-size: 59%;
		background-position: center;
		margin-top: -36.5vh;
	}

	.label {
		text-align: center;
		width: 100%;
		color: rgba(0, 0, 0, 0.6);
		font-size: 1.2rem;
		position: absolute;
		top: 115%;
	}

	.spinner {
		box-sizing: border-box;
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
