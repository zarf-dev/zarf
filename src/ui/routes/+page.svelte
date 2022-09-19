<script>
	import { clusterStore } from '$lib/store';

	import { goto } from '$app/navigation';
	import Spinner from '$lib/components/spinner.svelte';
	import bigZarf from '@images/zarf-bubbles.png';
	import { Button } from '@ui';
</script>

{#if $clusterStore}
	{#if $clusterStore.reachable}
		{#if $clusterStore.hasZarf}
			{goto(`/packages`, { replaceState: true })}
		{:else}
			<section class="hero">
				<div class="hero-content">
					<img src={bigZarf} alt="Zarf logo" id="zarf-logo" width="40%" />

					<div class="hero-text">
						<h1 class="hero-title">No Active Clusters</h1>

						<h2 class="hero-subtitle">
							Click initialize cluster to install the Init Package and deploy a new cluster.
						</h2>
					</div>

					<Button variant="raised" color="primary" href="/initialize/configure"
						>Initialize Cluster</Button
					>
				</div>
			</section>
		{/if}
	{/if}
{:else}
	<Spinner />
{/if}

<style>
	#zarf-logo {
		-webkit-transform: scaleX(-1);
		transform: scaleX(-1);
	}
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
	h1,
	h2 {
		margin: 0;
	}
	.hero-subtitle {
		font-size: large;
	}
	.hero-text {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		margin: 1rem 0;
	}
</style>
