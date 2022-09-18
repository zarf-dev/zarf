<script>
	import { fade } from 'svelte/transition';
	export let msg = 'Loading...';
</script>

<div class="spinner-wrapper" in:fade={{ duration: 200 }}>
	<span class="spinner" />
	<span class="label">{msg}</span>
</div>

<style lang="scss">
	.spinner-wrapper {
		display: block;
		position: relative;
		width: 1px;
		height: 1px;
		padding: 5rem;
		margin: 8rem auto;
	}

	.label {
		text-align: center;
		width: 100%;
		display: block;
		color: rgba(13, 19, 61, 1);
		font-size: 1.2rem;
		position: absolute;
		bottom: 0;
		left: 0;
	}

	.spinner::after {
		content: '';
		box-sizing: border-box;
		width: 5.5rem;
		height: 5.5rem;
		position: absolute;
		left: 2rem;
		top: 2rem;
		border-radius: 50%;
		border-top: 6px solid rgba(13, 19, 61, 1);
		border-left: 6px solid rgba(13, 19, 61, 1);
		border-bottom: 6px solid rgba(13, 19, 61, 1);
		border-right: 6px solid rgba(13, 19, 61, 0);
		animation: spinner 1s linear infinite;
	}

	@keyframes spinner {
		to {
			transform: rotate(360deg);
		}
	}
</style>
