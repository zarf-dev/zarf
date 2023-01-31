<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import {
		AccordionGroup,
		Icon,
		PackageDetailsCard as PackageDetails,
		PackageComponentAccordion as PackageComponent,
		PackageSectionHeader as SectionHeader,
		Divider
	} from '$lib/components';
	import { pkgStore } from '$lib/store';
	import { Button, Typography } from '@ui';
	import {page} from '$app/stores';
</script>

<svelte:head>
	<title>Configure</title>
</svelte:head>
<section class="page-header">
	<Typography variant="h5">Configure Package Deployment</Typography>
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
		<Typography variant="h5" slot="title">Components</Typography>
	</SectionHeader>
	<Typography variant="caption" element="p">
		<span aria-hidden="true">
			<Icon variant="component" class="invisible" />
		</span>
		The following components will be deployed into the cluster. Optional components that are not selected
		will not be deployed.
	</Typography>

	<AccordionGroup>
		{#each $pkgStore.zarfPackage.components as component, idx}
			<PackageComponent {idx} {component} readOnly={false} />
		{/each}
	</AccordionGroup>
</section>

<Divider />

<section class="actionButtonsContainer" aria-label="action buttons">
	<Button href="/" variant="outlined" color="secondary">cancel deployment</Button>
	<Button href={`/package/${$page.params.name}/review`} variant="raised" color="secondary">review deployment</Button>
</section>
