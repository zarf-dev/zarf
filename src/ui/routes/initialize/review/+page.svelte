<script lang="ts">
	import Icon from '$lib/components/icon.svelte';
	import PackageDetails from '$lib/components/package-details-card.svelte';
	import PackageComponent from '$lib/components/package-component-accordion.svelte';
	import AccordionGroup from '$lib/components/accordion-group.svelte';

	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Button, Typography } from '@ui';
</script>

<svelte:head>
	<title>Review</title>
</svelte:head>

<section class="pageHeader">
	<Typography variant="h4">Review Deployment</Typography>
</section>

<section class="initSection">
	<Typography variant="h5">
		<Icon variant="package" />
		Package Details
	</Typography>
	<PackageDetails pkg={$pkgStore.zarfPackage} />
</section>

<section class="initSection">
	<Typography variant="h5">
		<Icon variant="component" />
		Selected Package Components
	</Typography>
	<AccordionGroup>
		{#each $pkgComponentDeployStore as idx}
			<PackageComponent {idx} component={$pkgStore.zarfPackage.components[idx]} />
		{/each}
	</AccordionGroup>
</section>

<div class="actionButtonsContainer">
	<Button href="/initialize/configure" variant="outlined" color="secondary">configure</Button>
	<Button href="/initialize/deploy" variant="flat" color="secondary">deploy</Button>
</div>
