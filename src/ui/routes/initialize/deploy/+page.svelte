<script lang="ts">
	import { pkgStore, pkgPath, pkgComponentDeployStore} from '$lib/store';
	import { Stepper } from '@ui';
  import { Cluster } from '$lib/api';
  import type { ZarfDeployOptions } from '$lib/api-types';

    // @todo - update this view with the store interactions
	// $: componentsStepperList = componentsToDeploy.map((idx) => {
	// 	const config = pkgConfig.components[idx];
	// 	return {
	// 		title: 'Deploy ' + config.Name,
	// 		iconContent: String(idx + 2),
	// 		disabled: true,
	// 		variant: 'primary'
	// 	};
	// });


    let componentList: string = "";
    for (let i = 0; i < $pkgComponentDeployStore.length; i++) {
      componentList += $pkgStore.components[$pkgComponentDeployStore[i]].name + ",";
    };

    if (componentList.length > 1) {
      componentList = componentList.slice(0, -1)
    }




    const deployOptions: ZarfDeployOptions = {
      ApplianceMode: false,
      Components: componentList,
      NodePort: "",
      StorageClass: "",
      SGetKeyPath: "",
      Secret: "",
      PackagePath: $pkgPath[0] //TODO:
    };
</script>


<div>Deploying...</div>


{#await Cluster.initialize(deployOptions) then successful}
{successful}
<div>Finished deploying</div>

{/await}

<h1>Deploy Package - {$pkgStore.metadata?.name}</h1>
<div style="display:flex;justify-content:center;">
	<!-- <Stepper orientation="vertical" steps={componentsStepperList} /> -->
</div>
