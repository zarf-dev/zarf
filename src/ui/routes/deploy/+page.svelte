<script>
	import { Button, Stepper, Accordion } from '@ui';
	import Container from '$lib/components/container.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import Icon from '$lib/components/icon.svelte';
	import pkgConfig from '../packages/sample.json';
	let currentStep = 1;
	let config;

	let componentsToDeploy = [
		{
			title: 'Validate Configuration',
			iconContent: '1',
			disabled: true,
			variant: 'primary'
		}
	];

	const incrementStep = () => {
		currentStep++;
	};
	const decrementStep = () => {
		currentStep--;
	};
	const triggerFakeDeploy = async () => {
		const sleep = (delay) => new Promise((resolve) => setTimeout(resolve, delay));
		componentsToDeploy.forEach(async (element) => {
			element.iconContent = undefined;
			element.disabled = false;
			await sleep(3000);
		});
	};
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>

{#if currentStep < 4}
	<Container>
		<Stepper
			orientation="horizontal"
			steps={[
				{ title: 'Configure', iconContent: '1', disabled: currentStep !== 1, variant: 'primary' },
				{ title: 'Review', iconContent: '2', disabled: currentStep !== 2, variant: 'primary' },
				{ title: 'Deploy', iconContent: '3', disabled: currentStep !== 3, variant: 'primary' }
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
						<div>
							{componentConfig.Name}
							<span style="color: gray;"
								>{componentConfig.Required ? '(Required)' : '(Optional)'}</span
							>
						</div>
						<div
							style="max-width: 250px; white-space: nowrap;overflow: hidden;text-overflow: ellipsis"
						>
							{componentConfig.Description}
						</div>
					</div>
					<div slot="content">
						<pre>{JSON.stringify(componentConfig, null, 2)}</pre>
					</div>
				</Accordion>
			{/each}
			<div class="actionButtonsContainer">
				<Button href="/" variant="outlined">cancel deployment</Button>
				<Button on:click={incrementStep} variant="flat" disabled={currentStep === 3}
					>review deployment</Button
				>
			</div>
		{:else if currentStep === 2}
			<h1>Review Deployment</h1>
			<p>Edits to default configurations are highlighted</p>
			<h2><Icon variant="package" /> Package Details</h2>

			<PackageCard pkg={pkgConfig} />
			<h2><Icon variant="component" /> Package Components</h2>

			{#each pkgConfig.PackageYaml.Components as componentConfig, idx}
				accordion
			{/each}
			<div class="actionButtonsContainer">
				<Button on:click={decrementStep} variant="outlined">configure</Button>
				<Button on:click={incrementStep} variant="flat" disabled={currentStep === 3}>deploy</Button>
			</div>
		{:else}
			<h1>Deploy Package - {pkgConfig.PackageName}</h1>
			<Stepper orientation="vertical" steps={componentsToDeploy} />

			<div class="actionButtonsContainer">
				<Button on:click={decrementStep} variant="outlined">review</Button>
				<Button>FAKE START DEPLOY</Button>
			</div>
		{/if}
	</Container>
{:else}
	succcess!
{/if}

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
