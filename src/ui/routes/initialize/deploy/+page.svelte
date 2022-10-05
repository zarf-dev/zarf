<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		createComponentStepMap,
		getComponentStepMapComponents,
		getDeployedComponents,
		setStepSuccessful
	} from './deploy-utils';
	import { onMount } from 'svelte';
	import { Packages } from '$lib/api';
	import { Dialog, Stepper, Typography } from '@ui';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import type { ZarfDeployOptions } from '$lib/api-types';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/Step.svelte';

	const POLL_TIME = 5000;

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
	let dialogOpen = false;
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
		setTimeout(() => {
			goto('/packages');
		}, POLL_TIME);
	}
	$: if (successful) {
		dialogOpen = true;
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
<Dialog open={dialogOpen}>
	<section class="success-dialog" slot="content">
		<img class="zarf-logo" src={bigZarf} alt="zarf-logo" />
		<Typography variant="h6" style="color: var(--mdc-theme-on-primary)">
			Package Sucessfully Deployed
		</Typography>
		<Typography variant="body2">
			You will be automatically redirected to the deployment details page.
		</Typography>
	</section>
</Dialog>

<style>
	.deployment-steps {
		display: flex;
		justify-content: center;
	}
	.success-dialog {
		display: flex;
		padding: 24px 16px;
		width: 444px;
		height: 220.67px;
		text-align: center;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1rem;
	}
	.zarf-logo {
		width: 64px;
		height: 62.67px;
	}
</style>
