<section class="hero">
	<div class="hero-content">
		<slot />
	</div>
</section>

<style>
	.hero {
		position: relative;
		width: 100vw;
		height: 100vh;
		display: flex;
		justify-content: center;
		align-items: center;
	}
	.hero-content {
		position: relative;
		text-align: center;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}
	.hero-content :global(.hero-subtitle) {
		font-size: large;
	}
	.hero-content :global(.hero-text) {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		margin: 1rem 0;
	}
</style>
