<script>
	import { Button, Stepper } from '@ui';
	import Container from '$lib/components/container.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import Icon from '$lib/components/icon.svelte';
	import initConfig from '../packages/sample.json';
</script>

<svelte:head>
	<title>Deploy</title>
</svelte:head>

<Container>
	<Stepper
		orientation="horizontal"
		steps={[
			{ title: 'Configure', iconContent: '1', disabled: false, variant: 'primary' },
			{ title: 'Review', iconContent: '2', disabled: true },
			{ title: 'Deploy', iconContent: '3', disabled: true }
		]}
	/>
	<h1>Configure Package Deployment</h1>
	<h2><Icon variant="package" /> Package Details</h2>
	<PackageCard pkg={initConfig} />
</Container>

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
