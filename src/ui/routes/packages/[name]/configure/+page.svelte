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
	import { Button, Typography, AccordionGroup, currentTheme } from '@ui';
	import { page } from '$app/stores';
	import BuildProvidence from '$lib/components/build-providence.svelte';
	import DeploymentActions from '$lib/components/deployment-actions.svelte';
	import ButtonDense from '$lib/components/button-dense.svelte';
	import YamlDrawer from '$lib/components/yaml-drawer.svelte';

	let toggleDrawer: () => void;
</script>

<svelte:head>
	<title>Configure Deployment</title>
</svelte:head>
<Typography variant="h5">Configure Deployment</Typography>

<SectionHeader>
	Package Details
	<span slot="tooltip">At-a-glance simple metadata about the package</span>
	<ButtonDense slot="actions" variant="outlined" backgroundColor="white" on:click={toggleDrawer}>
		Code
	</ButtonDense>
</SectionHeader>
<PackageDetails pkg={$pkgStore.zarfPackage} />

<BuildProvidence build={$pkgStore.zarfPackage.build} />

<SectionHeader icon="cubes">
	Components
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
<YamlDrawer code={$pkgStore.zarfPackage} bind:toggleDrawer />
