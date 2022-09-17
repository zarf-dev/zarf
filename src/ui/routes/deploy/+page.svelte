<script lang="ts">
	import { Button, Stepper, Accordion, StepIcon } from '@ui';
	import Container from '$lib/components/container.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import Icon from '$lib/components/icon.svelte';
	import pkgConfig from '../packages/sample.json';
	import Header from '$lib/components/header.svelte';
	import Layout from '../+layout.svelte';
	import Modal from '$lib/components/modal.svelte';
	let currentStep = 3;

	type Step = {
		title: string;
		iconContent: string | undefined;
		disabled: boolean;
		variant: 'primary' | 'secondary';
	};

	const defaultStep1: Step = {
		title: 'Validate Config',
		iconContent: '1',
		disabled: false,
		variant: 'primary'
	};

	let componentsStepperList: Step[];

	let componentsToDeploy: number[] = pkgConfig.PackageYaml.Components.filter((c) => c.Required).map(
		(_, idx) => idx
	);

	const toggleComponentDeployment = (idx: number) => {
		const enabled = componentsToDeploy.includes(idx);
		if (enabled) {
			componentsToDeploy = [...componentsToDeploy].filter((n) => n !== idx);
		} else {
			componentsToDeploy = [...componentsToDeploy, idx];
		}
	};

	$: componentsStepperList = componentsToDeploy.map((idx) => {
		const config = pkgConfig.PackageYaml.Components[idx];
		return {
			title: 'Deploy ' + config.Name,
			iconContent: String(idx + 2),
			disabled: true,
			variant: 'primary'
		};
	});

	const incrementStep = () => {
		currentStep++;
	};
	const decrementStep = () => {
		currentStep--;
	};
	let deploymentStatus: string;
	const triggerFakeDeploy = async () => {
		const sleep = (delay: number) => new Promise((resolve) => setTimeout(resolve, delay));
		await sleep(1000);
		for (let i = 0; i < componentsStepperList.length; i++) {
			componentsStepperList[i].iconContent = undefined;
			componentsStepperList[i].disabled = false;
			await sleep(3000);
		}
		currentStep++;
		deploymentStatus = 'succeeded';
	};
	$: componentsToDeploy = componentsToDeploy.sort();
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>

{#if currentStep < 4}
	<Container>
		<Stepper
			orientation="horizontal"
			steps={[
				{
					title: 'Configure',
					iconContent: currentStep < 2 ? '1' : undefined,
					disabled: false,
					variant: 'primary'
				},
				{
					title: 'Review',
					iconContent: currentStep < 3 ? '2' : undefined,
					disabled: currentStep < 2,
					variant: 'primary'
				},
				{ title: 'Deploy', iconContent: '3', disabled: currentStep < 3, variant: 'primary' }
			]}
		/>

		{#if currentStep === 1}
			<h1>Configure Package Deployment</h1>
			<h2><Icon variant="package" /> Package Details</h2>

			<PackageCard pkg={pkgConfig} />

			<h2><Icon variant="component" /> Package Components</h2>

			{#each pkgConfig.PackageYaml.Components as componentConfig, idx}
				<Accordion id={`component-accordion-${idx}`}>
					<div slot="headerContent" class="component-accordion-header">
						<div style="display:flex;width: 60%;justify-content:space-between;">
							<div>
								{componentConfig.Name}
								{#if componentConfig.Required}
									<span style="color:gray;">(Required)</span>
								{:else}
									<span style="color:skyblue;">(Optional)</span>
								{/if}
							</div>
							<div
								style="max-width: 250px; white-space: nowrap;overflow: hidden;text-overflow: ellipsis"
							>
								{componentConfig.Description}
							</div>
						</div>

						<div>
							<input
								disabled={componentConfig.Required}
								checked={componentsToDeploy.includes(idx)}
								type="checkbox"
								id={`deploy-component-${idx}`}
								on:change={() => toggleComponentDeployment(idx)}
							/>
							<label
								style={componentConfig.Required ? 'color: #b1b1b1;' : ''}
								for={`deploy-component-${idx}`}>Deploy</label
							>
						</div>
					</div>
					<div slot="content">
						<pre>{JSON.stringify(componentConfig, null, 2)}</pre>
					</div>
				</Accordion>
				<br />
			{/each}
			<div class="actionButtonsContainer">
				<Button href="/" variant="outlined">cancel deployment</Button>
				<Button on:click={incrementStep} variant="flat">review deployment</Button>
			</div>
		{:else if currentStep === 2}
			<h1>Review Deployment</h1>
			<p>Edits to default configurations are highlighted</p>
			<h2><Icon variant="package" /> Package Details</h2>

			<PackageCard pkg={pkgConfig} />
			<h2><Icon variant="component" /> Package Components</h2>
			<ul>
				{#each componentsToDeploy as idx}
					<li>{pkgConfig.PackageYaml.Components[idx].Name} will be deployed</li>
				{/each}
			</ul>
			<div class="actionButtonsContainer">
				<Button on:click={decrementStep} variant="outlined">configure</Button>
				<Button on:click={incrementStep} variant="flat">deploy</Button>
			</div>
		{:else}
			<h1>Deploy Package - {pkgConfig.PackageName}</h1>
			<div style="display:flex;justify-content:center;">
				<Stepper orientation="vertical" steps={[...[defaultStep1], ...componentsStepperList]} />
			</div>
			<div class="actionButtonsContainer">
				<Button on:click={decrementStep} variant="outlined">review</Button>
				<Button on:click={triggerFakeDeploy}>FAKE START DEPLOY</Button>
			</div>
		{/if}
	</Container>
{:else if deploymentStatus === 'succeeded'}
	<Modal />
{/if}

<Modal />

<style>
	h1 {
		font-size: 34px;
		font-weight: 400;
		line-height: 42px;
		letter-spacing: 0.25px;
	}
	h2 {
		display: flex;
		gap: 0.75rem; /* 12px */
	}
	.actionButtonsContainer {
		display: flex;
		justify-content: space-between;
		margin-top: 2rem;
	}

	.component-accordion-header {
		display: flex;
		justify-content: space-between;
		width: 100%;
	}
	:global(.accordion-header) {
		width: 100%;
	}
</style>
