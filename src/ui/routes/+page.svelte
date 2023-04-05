<script lang="ts">
	import { Cluster } from '$lib/api';
	import type { ClusterSummary } from '$lib/api-types';
	import ClusterInfo from '$lib/components/cluster-info.svelte';
	import PackageTable from '$lib/components/package-table.svelte';
	import { clusterStore } from '$lib/store';
	import { onMount } from 'svelte';

	let clusterPoll: NodeJS.Timer;

	async function getClusterSummary(): Promise<void> {
		// Try to get the cluster summary
		Cluster.summary()
			// If success update the store
			.then((val: ClusterSummary) => {
				if (val.distro) {
					clusterStore.set(val);
					clearInterval(clusterPoll);
				}
				if (val.hasZarf) {
				}
			})
			.catch();
	}

	onMount(() => {
		getClusterSummary();
		clusterPoll = setInterval(getClusterSummary, 5000);
		return () => {
			clearInterval(clusterPoll);
		};
	});
</script>

<ClusterInfo />
<PackageTable />
