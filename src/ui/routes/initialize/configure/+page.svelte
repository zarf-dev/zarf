<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import AccordionGroup from '$lib/components/accordion-group.svelte';

	import Icon from '$lib/components/icon.svelte';
	import PackageComponent from '$lib/components/package-component-accordion.svelte';
	import { pkgStore } from '$lib/store';
	import { Button, Chip, Dialog, Typography } from '@ui';
	import SectionHeader from '$lib/components/pkg/section-header.svelte';
	let showRaw: boolean = false;
	let toggleShowRaw = () => (showRaw = !showRaw);
	import CodeBlock from '$lib/components/code-block.svelte';
	import { stringify } from 'yaml';
  	import Drawer from '$lib/components/drawer.svelte';
</script>

<svelte:head>
	<title>Configure</title>
</svelte:head>
<section class="page-header">
	<Typography variant="h2">Configure Deployment</Typography>
</section>

<section class="page-section">
	<SectionHeader>
		<Typography variant="h2" slot="title">Package Details</Typography>
		<span slot="tooltip">At-a-glance simple metadata about the package</span>
		<Button on:click={toggleShowRaw} variant="text" color="primary" slot="actions">view yaml</Button
		>
	</SectionHeader>
	<Drawer placement="right" size="fit-content" open={showRaw} on:clickAway={() => showRaw = false}>
		<CodeBlock language="yaml">{stringify($pkgStore.zarfPackage)}</CodeBlock>
	</Drawer>
	<div class="pkg-details">
		<div class="pkg-details-chips">
			<Typography variant="h2">
				{$pkgStore.zarfPackage.metadata?.name}
			</Typography>
			<Chip variant="filled">{$pkgStore.zarfPackage.metadata?.version}</Chip>
			<Chip variant="filled">{$pkgStore.zarfPackage.metadata?.architecture}</Chip>
			<Chip variant="filled">{$pkgStore.zarfPackage.kind}</Chip>
		</div>
		<Typography variant="body2">
			{$pkgStore.zarfPackage.metadata?.description}
		</Typography>
	</div>
</section>

<section class="page-section">
	<SectionHeader>
		<Typography variant="h2" slot="title">Supply Chain</Typography>
	</SectionHeader>
	<div style="margin-left: 2rem;">
		<Typography variant="h3">Build Providence</Typography>
		<table>
			<tr>
				<Typography variant="caption" element="td">User:</Typography>
				<Typography variant="th" element="td">{$pkgStore.zarfPackage.build?.user}</Typography>
			</tr>
			<tr>
				<Typography variant="caption" element="td">Terminal:</Typography>
				<Typography variant="th" element="td">{$pkgStore.zarfPackage.build?.terminal}</Typography>
			</tr>
			<tr>
				<Typography variant="caption" element="td">Timestamp:</Typography>
				<Typography variant="th" element="td">{$pkgStore.zarfPackage.build?.timestamp}</Typography>
			</tr>
		</table>
	</div>
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

<style>
	.pkg-details-chips {
		display: flex;
		gap: 1rem;
		align-items: center;
	}
	.pkg-details {
		padding: 0 3rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}
</style>
