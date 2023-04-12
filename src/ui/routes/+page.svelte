<script lang="ts">
	import { Cluster, Packages } from '$lib/api';
	import type { ClusterSummary, DeployedPackage } from '$lib/api-types';
	import ClusterInfo from '$lib/components/cluster-info.svelte';
	import PackageTable from '$lib/components/deployed-package-table.svelte';
	import { clusterStore, deployedPkgStore } from '$lib/store';
	import { onMount } from 'svelte';
	const POLL_TIME = 5000;
	let clusterPoll: NodeJS.Timer;
	let deployedPkgPoll: NodeJS.Timer;

	async function storeClusterSummary(): Promise<void> {
		// Try to get the cluster summary
		Cluster.summary()
			// If success update the store
			.then((val: ClusterSummary) => {
				clusterStore.set(val);
			})
			.catch(() => clusterStore.set(undefined));
	}
	async function storeDeployedPkgs(): Promise<void> {
		Packages.getDeployedPackages()
			.then((pkgs: DeployedPackage[]) => {
				deployedPkgStore.set({ pkgs });
			})
			.catch((err) => {
				if ($clusterStore) {
					deployedPkgStore.set({ err });
				} else {
					deployedPkgStore.set({ pkgs: [] });
				}
			});
	}

	onMount(() => {
		storeClusterSummary();
		storeDeployedPkgs();
		clusterPoll = setInterval(storeClusterSummary, POLL_TIME);
		deployedPkgPoll = setInterval(storeDeployedPkgs, POLL_TIME);
		return () => {
			clearInterval(clusterPoll);
			clearInterval(deployedPkgPoll);
		};
	});
</script>

<ClusterInfo />
<PackageTable />
