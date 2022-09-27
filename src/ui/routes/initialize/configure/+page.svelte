<script lang="ts">
	import Icon from '$lib/components/icon.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import PackageComponent from '$lib/components/package-component.svelte';
	import { pkgStore } from '$lib/store';
	import { Button } from '@ui';
</script>

<svelte:head>
	<title>Configure</title>
</svelte:head>

<h1>Configure Package Deployment</h1>
<h2><Icon variant="package" /> Package Details</h2>

<PackageCard pkg={$pkgStore.zarfPackage} />

<h2><Icon variant="component" /> Package Components</h2>

<div style="width: 100%;gap: 2px; display: flex; flex-direction: column;">
	{#each $pkgStore.zarfPackage.components as component, idx}
		<PackageComponent {idx} {component} readOnly={false} />
	{/each}
</div>

<div class="actionButtonsContainer">
	<Button href="/" variant="outlined">cancel deployment</Button>

	<Button href="/initialize/review" variant="raised">review deployment</Button>
</div>
