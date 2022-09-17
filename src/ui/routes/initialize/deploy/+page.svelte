<script lang="ts">
	import { pkgStore } from '$lib/store';
	import { Stepper } from '@ui';

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
</script>

<h1>Deploy Package - {$pkgStore.metadata?.name}</h1>
<div style="display:flex;justify-content:center;">
	<!-- <Stepper orientation="vertical" steps={componentsStepperList} /> -->
</div>
