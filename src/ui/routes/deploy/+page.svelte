<script>
	import Container from '$lib/components/container.svelte';
	import Icon from '$lib/components/icon.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import { Stepper } from '@ui';
	import initConfig from '../packages/sample.json';
	let currentStep = 1;

	const incrementStep = () => {
		currentStep++;
	};
	const decrementStep = () => {
		currentStep--;
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
		<h1>Configure Package Deployment</h1>
		<h2><Icon variant="package" /> Package Details</h2>

		<PackageCard pkg={initConfig} />

		<div style="display: flex; justify-content:space-between; margin-top: 2rem;">
			<Button href="/" variant="outlined">cancel deployment</Button>
			<Button on:click={incrementStep} variant="flat" disabled={currentStep === 3}
				>review deployment</Button
			>
		</div>
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
</style>
