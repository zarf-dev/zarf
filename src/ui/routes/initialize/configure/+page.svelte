<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import AccordionGroup from '../../../lib/components/accordion-group.svelte';

	import Icon from '$lib/components/icon.svelte';
	import PackageDetails from '$lib/components/package-details-card.svelte';
	import PackageComponent from '$lib/components/package-component-accordion.svelte';
	import { pkgStore } from '$lib/store';
	import { Button, Typography } from '@ui';
	import PackageCard from '$lib/components/package-card.svelte';
	import SectionHeader from '$lib/components/pkg/section-header.svelte';
</script>

<svelte:head>
	<title>Configure</title>
</svelte:head>
<section class="page-header">
	<Typography variant="h4">Configure Package Deployment</Typography>
</section>

<section class="page-section">
	<SectionHeader>
		<Typography variant="h2" slot="title">Package Details</Typography>
		<span slot="tooltip"
			>At-a-glance simple metadata about the package</span
		>
		<Button on:click={() => console.log("hello!")} variant="text" color="primary" slot="actions">click me</Button>
	</SectionHeader>
	<PackageCard pkg={$pkgStore.zarfPackage} />
</section>

<section class="page-section">
	<Typography variant="h5">
		<Icon variant="package" />
		Package Details
	</Typography>
	<PackageDetails pkg={$pkgStore.zarfPackage} />
</section>

<section class="page-section">
	<Typography variant="h5">
		<Icon variant="component" />
		Package Components
		<Typography variant="caption" element="p">
			The following components will be deployed into the cluster. Optional components that are not
			selected will not be deployed.
		</Typography>
	</Typography>

	<AccordionGroup>
		{#each $pkgStore.zarfPackage.components as component, idx}
			<PackageComponent {idx} {component} readOnly={false} />
		{/each}
	</AccordionGroup>
</section>

<section class="actionButtonsContainer" aria-label="action buttons">
	<Button href="/" variant="outlined" color="secondary">cancel deployment</Button>
	<Button href="/initialize/review" variant="raised" color="secondary">review deployment</Button>
</section>
