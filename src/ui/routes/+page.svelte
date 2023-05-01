<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Cluster, Packages, Tunnels } from '$lib/api';
	import type { ClusterSummary, DeployedPackage } from '$lib/api-types';
	import ClusterInfo from '$lib/components/cluster-info.svelte';
	import PackageTable from '$lib/components/deployed-package-table.svelte';
	import { clusterStore, deployedPkgStore, tunnelStore } from '$lib/store';
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
				// update the tunnel store with the new packages
				updateTunnels(pkgs);
			})
			.catch((err) => {
				if ($clusterStore) {
					deployedPkgStore.set({ err });
				} else {
					deployedPkgStore.set({ pkgs: [] });
				}
			});
	}

	// Retrieve the current tunnels and update the store by matching to package connect-strings
	async function updateTunnels(packages: DeployedPackage[]): Promise<void> {
		// Retrieve the current tunnel names
		const tunnels = await Tunnels.list();
		// Retrieve the current tunnel store
		const currentTunnels = { ...$tunnelStore };
		// For each package check if it has a tunnel and update the store
		for (const pkg of packages) {
			const packageConnections = await Packages.packageConnections(pkg.name);
			// Check if any of the tunnels match the package connect-strings
			const resource = tunnels.filter((tunnel) => {
				return Object.keys(packageConnections.connectStrings).includes(tunnel);
			});
			// If there is a match update the store clone
			if (resource.length > 0) {
				currentTunnels[pkg.name] = resource;
			} else {
				// If there is no match delete the package from the store clone
				delete currentTunnels[pkg.name];
			}
		}
		// Update the store
		tunnelStore.set(currentTunnels);
	}

	onMount(() => {
		const tunnels = localStorage.getItem('tunnels');
		if (tunnels) {
			tunnelStore.set(JSON.parse(tunnels));
		}
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
