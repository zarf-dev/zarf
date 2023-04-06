<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import {
		PackageDetailsCard as PackageDetails,
		PackageComponentAccordion as PackageComponent,
		PackageSectionHeader as SectionHeader,
		AccordionGroup,
		Divider,
	} from '$lib/components';

	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Button, Typography } from '@ui';
	import { page } from '$app/stores';
</script>

<svelte:head>
	<title>Review</title>
</svelte:head>

<section class="page-header">
	<Typography variant="h5">Review Deployment</Typography>
</section>

<section class="page-section">
	<SectionHeader>
		<Typography variant="h5" slot="title">Package Details</Typography>
		<span slot="tooltip">At-a-glance simple metadata about the package</span>
	</SectionHeader>
	<PackageDetails pkg={$pkgStore.zarfPackage} />
</section>

<section class="page-section">
	<SectionHeader>
		<Typography variant="h5" slot="title">Selected Package Components</Typography>
	</SectionHeader>
	<AccordionGroup>
		{#each $pkgComponentDeployStore as idx}
			<PackageComponent {idx} component={$pkgStore.zarfPackage.components[idx]} />
		{/each}
	</AccordionGroup>
</section>

<Divider />

<div class="actionButtonsContainer">
	<Button href={`/packages/${$page.params.name}/configure`} variant="outlined" color="secondary"
		>configure</Button
	>
	<Button href={`/packages/${$page.params.name}/deploy`} variant="flat" color="secondary">
		deploy
	</Button>
</div>
