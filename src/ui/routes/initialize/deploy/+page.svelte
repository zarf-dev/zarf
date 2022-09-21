<script lang="ts">
	import { Packages } from '$lib/api';
	import type { ZarfDeployOptions } from '$lib/api-types';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/Step.svelte';
	import { Stepper } from '@ui';

	let allComponents = $pkgStore.zarfPackage.components;
	let flatList: string[] = [];

	const components: StepProps[] = $pkgComponentDeployStore.map((idx) => {
		let config = allComponents[idx];
		flatList.push(config.name);
		return {
			name: config.name,
			title: 'Deploy ' + config.name,
			iconContent: String(idx),
			disabled: true,
			variant: 'primary'
		};
	});

	const deployOptions: ZarfDeployOptions = {
		applianceMode: false,
		components: flatList.join(','),
		nodePort: '',
		storageClass: '',
		sGetKeyPath: '',
		secret: '',
		packagePath: $pkgStore.path
	};
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>

{#await Packages.deploy(deployOptions)}
	<h1>Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</h1>
	<div style="display:flex;justify-content:center;">
		<Stepper orientation="vertical" steps={components} />
	</div>
{:then successful}
	<h1>Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</h1>
	<div style="display:flex; justify-content:center;">
		<Stepper
			orientation="vertical"
			steps={[
				...components.map((c) => {
					c.iconContent = undefined;
					c.variant = 'info';
					c.disabled = false;
					return c;
				}),
				{
					title: successful ? 'Deployment Succeeded' : 'Deployment Failed',
					variant: successful ? 'success' : 'error',
					disabled: false
				}
			]}
		/>
	</div>
{/await}
