<script>
	import { Packages } from '$lib/api';
	import Container from '$lib/components/container.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import Spinner from '$lib/components/spinner.svelte';
	import { Button } from '@ui';
</script>

{#await Packages.getDeployedPackages()}
	<Spinner />
{:then packages}
	<Container>
		<div class="top-title">
			<h1>📦 Deployed Zarf Packages</h1>
			<Button variant="outlined">✚ New Package</Button>
		</div>
		{#each packages as pkg}
			<article>
				<PackageCard pkg={pkg.data} />
			</article>
		{/each}
	</Container>
{/await}

<style>
	.top-title {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	article {
		margin: 1rem 0;
	}
</style>
