<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { Packages } from '$lib/api';
	import { pkgStore } from '$lib/store';
	import { PackageErrNotFound } from '$lib/components';

	enum LoadingStatus {
		Loading,
		Success,
		Error
	}
	let status: LoadingStatus = LoadingStatus.Loading;
	let errMessage: string = '';
	const pkgPath = $page.url.searchParams.get('path');
	let pkgName: string;

	if (pkgPath === null) {
		errMessage = 'No package path provided';
		status = LoadingStatus.Error;
	} else {
		pkgName = encodeURIComponent(
			pkgPath
				?.split('/')
				.at(-1)
				?.replaceAll('zarf-package-', '')
				.replaceAll('.tar', '')
				.replaceAll('.zst', '') ?? ''
		);

		if (pkgPath === 'init') {
			Packages.findInit()
				.catch(async (err: Error) => {
					errMessage = err.message;
					status = LoadingStatus.Error;
				})
				.then((res) => {
					if (Array.isArray(res)) {
						Packages.read(res[0])
							.then(pkgStore.set)
							.then(() => {
								status = LoadingStatus.Success;
							});
					}
				});
		} else {
			Packages.read(pkgPath)
				.then(pkgStore.set)
				.then(() => {
					status = LoadingStatus.Success;
				})
				.catch(async (err: Error) => {
					errMessage = err.message;
					status = LoadingStatus.Error;
				});
		}
	}
</script>

{#if status == LoadingStatus.Loading}
	<div>loading...</div>
{:else if status == LoadingStatus.Error}
	<PackageErrNotFound pkgName={errMessage} />
{:else}
	{goto(`/package/${pkgName}/configure`)}
{/if}
