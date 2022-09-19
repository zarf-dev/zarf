<script>
	import bigZarf from '@images/zarf-bubbles-right.png';
	import { fade } from 'svelte/transition';
	import Hero from './hero.svelte';

	export let msg = 'Loading...';
</script>

<Hero>
	<div
		class="spinner-wrapper"
		style="background-image: url('{bigZarf}')"
		in:fade={{ duration: 200 }}
	>
		<span class="spinner" />
		<span class="label">{msg}</span>
	</div>
</Hero>

<style lang="scss">
	.spinner-wrapper {
		display: flex;
		background-size: 59%;
		background-position: center;
		margin-top: -11.95rem;
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
