<script>
	import { Cluster } from '$lib/api';
	import bigZarf from '@images/zarf-bubbles.png';
	import { Button } from '@ui';
	import Spinner from '$lib/components/spinner.svelte';
</script>

<svelte:head>
	<title>Zarf</title>
</svelte:head>

{#await Cluster.summary()}
	<section class="hero">
		<div class="hero-content">
			<Spinner />
		</div>
	</section>
{:then summary}
	{#if summary.reachable}
		{#if summary.hasZarf}
			<section class="hero">
				<div class="hero-content">REPLACE_ME_HAZ_CLUSTER</div>
			</section>
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

					<Button variant="raised" color="primary" href="/initialize/configure">Initialize Cluter</Button>
				</div>
			</section>
		{/if}
	{/if}
{/await}

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
