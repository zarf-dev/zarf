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

	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Button, AccordionGroup, currentTheme } from '@ui';
	import { page } from '$app/stores';
	import BuildProvidence from '$lib/components/build-providence.svelte';
	import DeploymentActions from '$lib/components/deployment-actions.svelte';
</script>

<svelte:head>
	<title>Review</title>
</svelte:head>

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

<SectionHeader>Selected Package Components</SectionHeader>
<AccordionGroup elevation={1}>
	{#each $pkgComponentDeployStore as idx}
		<PackageComponent {idx} component={$pkgStore.zarfPackage.components[idx]} />
	{/each}
</AccordionGroup>

<Divider />

<DeploymentActions>
	<Button
		href={`/packages/${$page.params.name}/configure`}
		backgroundColor={$currentTheme === 'light' ? 'black' : 'grey-300'}
		variant="outlined">configure</Button
	>
	<Button
		href={`/packages/${$page.params.name}/deploy`}
		backgroundColor="grey-300"
		textColor="black"
		variant="raised"
	>
		deploy
	</Button>
</DeploymentActions>
