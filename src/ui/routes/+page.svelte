<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script>
	import { goto } from '$app/navigation';
	import { Button, Typography } from '@ui';
	import { clusterStore } from '$lib/store';
	import { Hero, Spinner } from '$lib/components';
	import bigZarf from '@images/zarf-bubbles-right.png';
</script>

<svelte:head>
	<title>Zarf UI</title>
</svelte:head>

{#if $clusterStore}
	{#if $clusterStore.hasZarf}
		{goto(`/packages`, { replaceState: true })}
	{:else}
		<Hero>
			<img src={bigZarf} alt="Zarf logo" id="zarf-logo" width="40%" />

			<Typography variant="h5" class="hero-title">No Active Zarf Clusters</Typography>

			{#if $clusterStore.reachable && $clusterStore.distro !== 'unknown'}
				<Typography variant="body2" class="hero-subtitle">
					A {$clusterStore.distro} cluster was found, click initialize cluster to initialize it now with
					Zarf.
				</Typography>
			{:else}
				<Typography variant="body2" class="hero-subtitle">
					Click initialize cluster to install the Init Package and deploy a new cluster.
				</Typography>
			{/if}

			<Button variant="raised" color="secondary" href="/package/deploy?path=init" id="init-cluster">
				Initialize Cluster
			</Button>
		</Hero>
	{/if}
{:else}
	<Spinner
	    title="Checking for cluster"
		msg="Checking if a Kubernetes cluster is available and initialized by Zarf. This may take a few seconds."
	/>
{/if}

<style>
	:global(#init-cluster) {
		margin-top: 1.5rem;
	}
</style>
