<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import {
		PackageDetailsCard as PackageDetails,
		PackageComponentAccordion as PackageComponent,
		PackageSectionHeader as SectionHeader,
		Divider,
	} from '$lib/components';
	import { pkgStore } from '$lib/store';
	import { Button, Paper, Typography, AccordionGroup, currentTheme } from '@ui';
	import { page } from '$app/stores';
	import BuildProvidence from '$lib/components/build-providence.svelte';
	import DeploymentActions from '$lib/components/deployment-actions.svelte';
</script>

<svelte:head>
	<title>Configure</title>
</svelte:head>
<Typography variant="h5">Configure Deployment</Typography>

<SectionHeader>
	Package Details
	<span slot="tooltip">At-a-glance simple metadata about the package</span>
</SectionHeader>
<PackageDetails pkg={$pkgStore.zarfPackage} />

<SectionHeader icon="secured_layer">
	Supply Chain
	<span slot="tooltip"
		>Supply chain is used to help determine if a package can be trusted. It includes declarative
		data regarding how this package was built. Build providence includes metadata about the build
		and where the package was created. SBOM includes information on all of the code, images, and
		resources contained in this package.
	</span>
</SectionHeader>
<BuildProvidence build={$pkgStore.zarfPackage.build} />

<SectionHeader
	>Components
	<span slot="tooltip">A set of defined functionality and resources that build up a package.</span>
</SectionHeader>

<AccordionGroup elevation={1}>
	{#each $pkgStore.zarfPackage.components as component, idx}
		<PackageComponent {idx} {component} readOnly={false} />
	{/each}
</AccordionGroup>

<Divider />

<DeploymentActions>
	<Button
		href="/"
		variant="outlined"
		color="secondary"
		backgroundColor={$currentTheme === 'light' ? 'black' : 'grey-300'}
	>
		cancel deployment
	</Button>
	<Button
		href={`/packages/${$page.params.name}/review`}
		variant="raised"
		backgroundColor="grey-300"
		textColor="black"
	>
		review deployment
	</Button>
</DeploymentActions>
