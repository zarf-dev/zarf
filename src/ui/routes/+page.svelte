<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import ClusterInfo from '$lib/components/cluster-info.svelte';
	import PackageTable from '$lib/components/deployed-package-table.svelte';
	import { updateClusterSummary, updateConnections, updateDeployedPkgs } from '$lib/store';
	import { onMount } from 'svelte';
	const POLL_TIME = 5000;

	onMount(() => {
		updateConnections();
		updateClusterSummary();
		updateDeployedPkgs();
		const clusterPoll = setInterval(updateClusterSummary, POLL_TIME);
		const deployedPkgPoll = setInterval(updateDeployedPkgs, POLL_TIME);
		const tunnelPoll = setInterval(updateConnections, POLL_TIME);
		return () => {
			clearInterval(clusterPoll);
			clearInterval(deployedPkgPoll);
			clearInterval(tunnelPoll);
		};
	});
</script>

<ClusterInfo />
<PackageTable />
