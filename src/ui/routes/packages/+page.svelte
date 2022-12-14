<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script>
	import { Packages } from '$lib/api';
	import Hero from '$lib/components/hero.svelte';
	import { DetailsCard } from '$lib/components/package';
	import Spinner from '$lib/components/spinner.svelte';
	import { Button, Typography } from '@ui';
	import Icon from '$lib/components/icon.svelte';
</script>

{#await Packages.getDeployedPackages()}
	<Spinner />
{:then packages}
	{#if packages.length < 1}
		<Hero>
			<div>
				<h3>No deployed packages found 🙁</h3>
				<Button href="/" variant="flat" color="secondary">Go Home</Button>
			</div>
		</Hero>
	{:else}
		<section class="page-title deployed-packages">
			<Typography variant="h5">Deployment Details</Typography>
			<Button variant="raised" color="secondary">
				<Icon variant="rocket" />
				Deploy Package
			</Button>
		</section>
		{#each packages as pkg}
			<section class="page-section">
				<Typography variant="h6">
					<Icon variant="package" />
					Deployed Packages
				</Typography>
				<DetailsCard pkg={pkg.data} />
			</section>
		{/each}
	{/if}
{/await}

<style>
	.deployed-packages {
		width: 100%;
		display: flex;
		justify-content: space-between;
	}
</style>
