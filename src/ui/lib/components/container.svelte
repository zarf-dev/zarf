<div class="container">
	<slot />
</div>

<style>
	.container {
		margin-left: auto;
		margin-right: auto;
	}
	@media (min-width: 768px) {
		.container {
			max-width: 48rem;
		}
	}
	@media (min-width: 1024px) {
		.container {
			max-width: 56rem;
		}
	}
</style>
