<script lang="ts">
	import {
		createComponentStepMap,
		getComponentStepMapComponents,
		getDeployedComponents,
		setStepSuccessful
	} from './deploy-utils';
	import { onMount } from 'svelte';
	import { Packages } from '$lib/api';
	import { Stepper, Typography } from '@ui';
	import type { ZarfDeployOptions } from '$lib/api-types';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/Step.svelte';

	const POLL_TIME = 2000;

	const components: Map<string, StepProps> = createComponentStepMap(
		$pkgStore.zarfPackage.components,
		$pkgComponentDeployStore
	);
	const deployOptions: ZarfDeployOptions = {
		applianceMode: false,
		components: Array.from(components.keys()).join(','),
		nodePort: '',
		storageClass: '',
		sGetKeyPath: '',
		secret: '',
		packagePath: $pkgStore.path
	};

	let successful = false;
	let finishedDeploying = false;
	let pollDeployed: NodeJS.Timer;
	let componentSteps: StepProps[] = getComponentStepMapComponents(components);

	async function updateComponentSteps(): Promise<void> {
		return getDeployedComponents(components).then((value: StepProps[]) => {
			componentSteps = value;
		});
	}

	onMount(() => {
		Packages.deploy(deployOptions).then(
			(value: boolean) => {
				finishedDeploying = true;
				successful = value;
			},
			() => {
				finishedDeploying = true;
			}
		);

		pollDeployed = setInterval(() => {
			updateComponentSteps();
		}, POLL_TIME);
		return () => {
			clearInterval(pollDeployed);
		};
	});

	$: if (finishedDeploying) {
		pollDeployed && clearInterval(pollDeployed);
		componentSteps = [
			...componentSteps.map((step: StepProps): StepProps => setStepSuccessful(step)),
			{
				title: successful ? 'Deployment Succeeded' : 'Deployment Failed',
				variant: successful ? 'success' : 'error',
				disabled: false
			}
		];
	}
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>
<section class="pageHeader">
	<Typography variant="h4">Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</Typography>
</section>
<section class="deployment-steps">
	<Stepper orientation="vertical" steps={componentSteps} />
</section>

<style>
	.deployment-steps {
		display: flex;
		justify-content: center;
	}
</style>
