<script>
	import { clusterStore } from '$lib/store';

	import { goto } from '$app/navigation';
	import Spinner from '$lib/components/spinner.svelte';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import { Button } from '@ui';
	import Hero from '$lib/components/hero.svelte';
</script>

{#if $clusterStore}
	{#if $clusterStore.hasZarf}
		{goto(`/packages`, { replaceState: true })}
	{:else}
		<Hero>
			<img src={bigZarf} alt="Zarf logo" id="zarf-logo" width="40%" />

			<div class="hero-text">
				<h1 class="hero-title">No Active Zarf Clusters</h1>

				{#if $clusterStore.reachable && $clusterStore.distro !== 'unknown'}
					<h2 class="hero-subtitle">
						A {$clusterStore.distro} cluster was found, click initialize cluster to initialize it now
						with Zarf.
					</h2>
				{:else}
					<h2 class="hero-subtitle">
						Click initialize cluster to install the Init Package and deploy a new cluster.
					</h2>
				{/if}
			</div>

			<Button variant="raised" color="primary" href="/initialize/configure"
				>Initialize Cluster</Button
			>
		</Hero>
	{/if}
{:else}
	<Spinner
		msg="Checking if a Kubernetes cluster is available and initialized by Zarf. This may take a seconds."
	/>
{/if}

<style>
	h1,
	h2 {
		margin: 0;
	}
</style>
