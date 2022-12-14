<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import Icon from '$lib/components/icon.svelte';
	import { DetailsCard, ComponentAccordion as PackageComponent } from '$lib/components/package';
	import AccordionGroup from '$lib/components/accordion-group.svelte';

	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Button, Typography } from '@ui';
</script>

<svelte:head>
	<title>Review</title>
</svelte:head>

<section class="page-header">
	<Typography variant="h5">Review Deployment</Typography>
</section>

<section class="page-section">
	<Typography variant="h5">
		<Icon variant="package" />
		Package Details
	</Typography>
	<DetailsCard pkg={$pkgStore.zarfPackage} />
</section>

<section class="page-section">
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
