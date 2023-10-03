<!--
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import type { APIZarfDeployPayload, ZarfPackageOptions, ZarfInitOptions } from '$lib/api-types';
	import { Dialog, Stepper, Typography, type StepProps, Box } from '@ui';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import { beforeNavigate, goto } from '$app/navigation';
	import { Packages } from '$lib/api';
	import { onMount } from 'svelte';
	import {
		getDialogContent,
		finalizeStepState,
		getDeployedComponents,
		createComponentStepMap,
		getComponentStepMapComponents,
		type DeployedSteps,
	} from './deploy-utils';
	import AnsiDisplay from '../../../../lib/components/ansi-display.svelte';
	import DeploymentActions from '$lib/components/deployment-actions.svelte';
	import ButtonDense from '$lib/components/button-dense.svelte';

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

	let options: APIZarfDeployPayload = {
		packageOpts: {
			optionalComponents: requestedComponents,
			sGetKeyPath: '',
			packageSource: $pkgStore.path,
			setVariables: {},
			publicKeyPath: '',
			shasum: '',
		},
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
				internalServer: true,
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
				secret: '',
			},
		} as ZarfInitOptions;
	}

	let activeIndex = 0;
	let hasError = false;
	let successful = false;
	let dialogOpen = false;
	let deployStream: AbortController;
	let finishedDeploying = false;
	let pollDeployed: NodeJS.Timer;
	let addMessage: (message: string) => void;
	let componentSteps: StepProps[] = getComponentStepMapComponents(components);
	let dialogState: { topLine: string; bottomLine: string } = getDialogContent(successful);

	async function updateComponentSteps(): Promise<void> {
		if (!$pkgStore.zarfPackage.metadata?.name) {
			return;
		}
		return getDeployedComponents($pkgStore.zarfPackage.metadata.name, components).then(
			(value: DeployedSteps) => {
				componentSteps = value.steps;
				activeIndex = value.activeStep;
			}
		);
	}

	beforeNavigate(() => {
		// Kill the stream and polling before navigating away
		deployStream?.abort();
		clearInterval(pollDeployed);
	});

	onMount(async () => {
		// Set up the log stream
		deployStream = Packages.deployStream({
			onmessage: (e) => {
				addMessage(e.data);
				if (e.data.includes('WARNING') || e.data.includes('ERROR')) {
					hasError = true;
					if (e.data.includes('ERROR')) {
						componentSteps[activeIndex].variant = 'error';
						componentSteps[activeIndex].subtitle = 'Error: See log stream for details.';
					} else {
						componentSteps[activeIndex].variant = 'warning';
						componentSteps[activeIndex].subtitle = 'Warning: See log stream for details.';
					}
					componentSteps[activeIndex].iconContent = '';
					componentSteps = [...componentSteps];
				}
			},
			onerror: (e) => {
				// Add the error message to the log stream
				addMessage(e.message);

				// Set the error state.
				successful = false;
				finishedDeploying = true;
			},
		});

		// Deploy the package
		Packages.deploy(options).then(
			(value: boolean) => {
				finishedDeploying = true;
				successful = value;
			},
			() => {
				successful = false;
				finishedDeploying = true;
			}
		);

		// Poll for deployed components
		pollDeployed = setInterval(() => {
			updateComponentSteps();
		}, POLL_TIME);

		// Kill the stream and polling onDestroy
		return () => {
			deployStream.abort();
			clearInterval(pollDeployed);
		};
	});

	$: if (finishedDeploying) {
		// Kill the stream and polling
		deployStream?.abort();
		pollDeployed && clearInterval(pollDeployed);

		// set all steps to success or error based on successful
		componentSteps = [
			...finalizeStepState(componentSteps, successful),
			// Add the success/failure step
			{
				title: successful ? 'Deployment Succeeded' : 'Deployment Failed',
				variant: successful ? 'success' : 'error',
				disabled: false,
			},
		];
		if (successful) {
			dialogOpen = true;
			dialogState = getDialogContent(successful);
			setTimeout(() => {
				goto('/');
			}, POLL_TIME);
		}
	}
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>
<section class="page-header">
	<Typography variant="h5">Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</Typography>
</section>
<Box
	ssx={{
		$self: {
			display: 'flex',
			gap: '240px',
			justifyContent: 'space-between',
			'& > .stepper': {
				minWidth: '240px',
			},
			// TODO: Remove this once the Stepper component is fixed to update colors correctly.
			// link to issue: https://github.com/defenseunicorns/UnicornUI/issues/229
			'& .step-icon.error': {
				backgroundColor: 'var(--error)',
			},
			'& .step-icon.success': {
				backgroundColor: 'var(--success)',
			},
			'& .step-icon.warning': {
				backgroundColor: 'var(--warning)',
			},
		},
	}}
	class="deployment-steps"
>
	<Stepper orientation="vertical" color="on-background" steps={[...componentSteps]} />
	<AnsiDisplay minWidth="100ch" bind:addMessage />
</Box>
{#if finishedDeploying && hasError}
	<DeploymentActions>
		<ButtonDense
			style="margin-left: auto;"
			variant="raised"
			backgroundColor="white"
			on:click={() => goto('/')}
		>
			Return to Packages
		</ButtonDense>
	</DeploymentActions>
{/if}
<Dialog open={dialogOpen}>
	<section class="success-dialog" slot="content">
		<img class="zarf-logo" src={bigZarf} alt="zarf-logo" />
		<Typography variant="h6" color="on-background">
			{dialogState.topLine}
		</Typography>
		<Typography variant="body2">
			{dialogState.bottomLine}
		</Typography>
	</section>
</Dialog>

<style>
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
