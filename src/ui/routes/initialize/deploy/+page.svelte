<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		createComponentStepMap,
		finalizeStepState,
		getComponentStepMapComponents,
		getDeployedComponents,
		getDialogContent
	} from './deploy-utils';
	import { onMount } from 'svelte';
	import { Packages } from '$lib/api';
	import { Dialog, Stepper, Typography } from '@ui';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import type { ZarfDeployOptions, ZarfInitOptions } from '$lib/api-types';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import type { StepProps } from '@defense-unicorns/unicorn-ui/Stepper/Step.svelte';

	const POLL_TIME = 5000;

	const components: Map<string, StepProps> = createComponentStepMap(
		$pkgStore.zarfPackage.components,
		$pkgComponentDeployStore
	);
	// comma-delimited string that contains only optional components that were enabled via UI
	const requestedComponents: string = $pkgStore.zarfPackage.components
		.filter((c, idx) => $pkgComponentDeployStore.includes(idx) && !c.required)
		.map((c) => c.name)
		.join(',');

	const isInitPkg = $pkgStore.zarfPackage.kind === 'ZarfInitConfig';
	
	type DeployPayloadBody = {
		initOpts?: ZarfInitOptions;
		deployOpts: ZarfDeployOptions;
	}

	let options: DeployPayloadBody = {
		deployOpts: {
			components: requestedComponents,
			sGetKeyPath: '',
			packagePath: $pkgStore.path,
			setVariables: {},
			insecure: false,
		  // "as" will cause the obj to satisfy the type
		  // it is missing "shasum"
		} as ZarfDeployOptions
	};

	if (isInitPkg) {
		options.initOpts = {
			applianceMode: false,
			gitServer: {
				address: '',
				pushUsername: 'zarf-git-user',
				pushPassword: '',
				pullUsername: 'zarf-git-read-user',
				pullPassword: '',
				internalServer: true
			},
			storageClass: '',
			registryInfo: {
				address: '',
				internalRegistry: true,
				nodePort: 0,
				pullPassword: '',
				pullUsername: 'zarf-pull',
				pushPassword: '',
				pushUsername: 'zarf-push',
				secret: ''
			}
		};
	}

	let successful = false;
	let finishedDeploying = false;
	let dialogOpen = false;
	let pollDeployed: NodeJS.Timer;
	let componentSteps: StepProps[] = getComponentStepMapComponents(components);
	let dialogState: { topLine: string; bottomLine: string } = getDialogContent(successful);

	async function updateComponentSteps(): Promise<void> {
		return getDeployedComponents(components).then((value: StepProps[]) => {
			componentSteps = value;
		});
	}

	onMount(() => {
		console.log('deploying', options)
		Packages.deploy(options).then(
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
			...finalizeStepState(componentSteps, successful),
			{
				title: successful ? 'Deployment Succeeded' : 'Deployment Failed',
				variant: successful ? 'success' : 'error',
				disabled: false
			}
		];
		dialogOpen = true;
		if (successful) {
			setTimeout(() => {
				goto('/packages');
			}, POLL_TIME);
		} else {
			setTimeout(() => {
				goto('/');
			}, POLL_TIME);
		}
	}
	$: if (successful) {
		dialogState = getDialogContent(successful);
	}
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>
<section class="page-header">
	<Typography variant="h4">Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</Typography>
</section>
<section class="deployment-steps">
	<Stepper orientation="vertical" steps={componentSteps} />
</section>
<Dialog open={dialogOpen}>
	<section class="success-dialog" slot="content">
		<img class="zarf-logo" src={bigZarf} alt="zarf-logo" />
		<Typography variant="h6" style="color: var(--mdc-theme-on-primary)">
			{dialogState.topLine}
		</Typography>
		<Typography variant="body2">
			{dialogState.bottomLine}
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
