<script lang="ts">
	import AccordionGroup from '../../../lib/components/accordion-group.svelte';

	import Icon from '$lib/components/icon.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import PackageComponent from '$lib/components/package-component.svelte';
	import { pkgStore } from '$lib/store';
	import { Button, Typography } from '@ui';
</script>

<svelte:head>
	<title>Configure</title>
</svelte:head>
<section class="pageHeader" style="margin-top: 2rem" aria-label="Page Title">
	<Typography variant="h4">Configure Package Deployment</Typography>
</section>

<section class="initSection" aria-label="Package Details">
	<Typography variant="h5">
		<Icon variant="package" />
		Package Details
	</Typography>
	<PackageCard pkg={$pkgStore.zarfPackage} />
</section>

<section class="initSection" aria-label="Package Components">
	<Typography variant="h5">
		<Icon variant="component" />
		Package Components
		<Typography variant="caption" element="p">
			<Icon variant="component" className="invisible" />
			The following components wil be deployed into the cluster. Optional components that are not selected
			will not be deployed.
		</Typography>
	</Typography>

	<AccordionGroup>
		{#each $pkgStore.zarfPackage.components as component, idx}
			<PackageComponent {idx} {component} readOnly={false} />
		{/each}
	</AccordionGroup>
</section>

<section class="actionButtonsContainer" aria-label="action buttons">
	<Button href="/" variant="outlined" shape="squared">cancel deployment</Button>
	<Button href="/initialize/review" variant="raised" shape="squared">review deployment</Button>
</section>

<style>
	.initSection {
		gap: 20px;
		display: flex;
		flex-direction: column;
	}
</style>
