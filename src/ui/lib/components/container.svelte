<div class="container">
	<slot />
</div>

<style>
	.container {
		margin-left: auto;
		margin-right: auto;
		margin-bottom: 3rem;
	}
	@media (min-width: 768px) {
		.container {
			max-width: 56rem;
		}
	}
	@media (min-width: 1200px) {
		.container {
			max-width: 66rem;
		}
	}
</style>
