<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import { pkgStore } from '$lib/store';
	import { PackageErrNotFound, Spinner } from '$lib/components';

	const pkgPath = $page.url.searchParams.get('path');

	const loadPkg = async (path: string) => {
		await Packages.read(path).then(pkgStore.set);
	};
</script>

{#if pkgPath === null}
	<PackageErrNotFound message="No package path provided" />
{:else}
	{#await loadPkg(pkgPath)}
		<Spinner title="Retrieving package" />
	{:then}
		<div class="invisible">
			{goto(`/package/${$pkgStore.zarfPackage.metadata?.name}/configure`)}
		</div>
	{:catch err}
		<PackageErrNotFound message={err.message} />
	{/await}
{/if}
