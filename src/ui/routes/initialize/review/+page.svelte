<script lang="ts">
	import Icon from '$lib/components/icon.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import PackageComponent from '$lib/components/package-component.svelte';

	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Button } from '@ui';
</script>

<h1>Review Deployment</h1>
<p>Edits to default configurations are highlighted</p>
<h2><Icon variant="package" /> Package Details</h2>

<PackageCard pkg={$pkgStore.zarfPackage} />
<h2><Icon variant="component" />Selected Package Components</h2>

<div style="width: 100%;gap: 2px; display: flex; flex-direction: column;">
	{#each $pkgComponentDeployStore as idx}
		<PackageComponent {idx} component={$pkgStore.zarfPackage.components[idx]} />
	{/each}
</div>

<div class="actionButtonsContainer">
	<Button href="/initialize/configure" variant="outlined">configure</Button>
	<Button href="/initialize/deploy" variant="flat">deploy</Button>
</div>
