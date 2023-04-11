<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import type { APIZarfDeployPayload, ZarfDeployOptions } from '$lib/api-types';
	import { Dialog, Stepper, Typography, type StepProps } from '@ui';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import { FitAddon } from 'xterm-addon-fit';
	import { goto } from '$app/navigation';
	import { Packages } from '$lib/api';
	import { onMount } from 'svelte';
	import { Terminal } from 'xterm';
	import 'xterm/css/xterm.css';
	import {
		getDialogContent,
		finalizeStepState,
		getDeployedComponents,
		createComponentStepMap,
		getComponentStepMapComponents,
	} from './deploy-utils';

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
		deployOpts: {
			components: requestedComponents,
			sGetKeyPath: '',
			packagePath: $pkgStore.path,
			setVariables: {},
			insecure: false,
			// "as" will cause the obj to satisfy the type
			// it is missing "shasum"
		} as unknown as ZarfDeployOptions,
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
		const deployStream = Packages.deployStream();
		const term = new Terminal({
			disableStdin: true,
			convertEol: true,
			customGlyphs: true,
			theme: { background: '#1E1E1E' },
		});
		const fitAddon = new FitAddon();
		term.loadAddon(fitAddon);

		const termElement = document.getElementById('terminal');
		if (termElement) {
			term.open(termElement);
			fitAddon.fit();
		}
		deployStream.addEventListener('message', (e: MessageEvent<string>) => {
			term.writeln(e.data);
		});
		Packages.deploy(options).then(
			(value: boolean) => {
				deployStream.close();
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
			deployStream.close();
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
				disabled: false,
			},
		];
		dialogOpen = true;
		setTimeout(() => {
			goto('/');
		}, POLL_TIME);
	}
	$: if (successful) {
		dialogState = getDialogContent(successful);
	}
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>
<section class="page-header">
	<Typography variant="h5">Deploy Package - {$pkgStore.zarfPackage.metadata?.name}</Typography>
</section>
<section class="deployment-steps">
	<Stepper orientation="vertical" color="on-background" steps={componentSteps} />
	<div id="terminal" />
</section>
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
	.deployment-steps {
		display: flex;
		justify-content: space-evenly;
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

	#terminal {
		width: 751px;
		height: 688px;
		padding: 8px;
	}
</style>
