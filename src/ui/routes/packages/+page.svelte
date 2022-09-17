<script>
	import Container from '$lib/components/container.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import { Button } from '@ui';
</script>

<svelte:head>
	<title>Packages</title>
</svelte:head>

<Container>
	<div class="top-title">
		<h1>📦 Zarf Packages</h1>
		<Button variant="outlined">✚ New Package</Button>
	</div>
	{#each data as pkgData}
		<article>
			<PackageCard pkg={pkgData} />
		</article>
	{/each}
</Container>

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
