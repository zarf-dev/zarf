<script lang="ts">
	import { pkgStore, pkgComponentDeployStore } from '$lib/store';
	import { Stepper } from '@ui';
	import { Cluster } from '$lib/api';
	import type { ZarfDeployOptions } from '$lib/api-types';
	import Spinner from '$lib/components/spinner.svelte';

	// $: componentsStepperList = componentsToDeploy.map((idx) => {
	// 	const config = pkgConfig.components[idx];
	// 	return {
	// 		title: 'Deploy ' + config.Name,
	// 		iconContent: String(idx + 2),
	// 		disabled: true,
	// 		variant: 'primary'
	// 	};
	// });

	let componentList: string = '';
	for (let i = 0; i < $pkgComponentDeployStore.length; i++) {
		componentList += $pkgStore.zarfPackage.components[$pkgComponentDeployStore[i]].name + ',';
	}

	if (componentList.length > 1) {
		componentList = componentList.slice(0, -1);
	}

	const deployOptions: ZarfDeployOptions = {
		applianceMode: false,
		components: componentList,
		nodePort: '',
		storageClass: '',
		sGetKeyPath: '',
		secret: '',
		packagePath: $pkgStore.path
	};
</script>

<div>Deploying...</div>

{#await Cluster.initialize(deployOptions)}
	<Spinner />
{:then successful}
	{successful}
	<div>Finished deploying</div>
{/await}

<h1>Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</h1>
<div style="display:flex;justify-content:center;">
	<!-- <Stepper orientation="vertical" steps={componentsStepperList} /> -->
</div>
