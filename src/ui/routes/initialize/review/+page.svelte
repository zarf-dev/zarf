<script lang="ts">
	import Icon from '$lib/components/icon.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import { pkgComponentDeployStore, pkgStore } from '$lib/store';
	import { Button } from '@ui';
</script>

<h1>Review Deployment</h1>
<p>Edits to default configurations are highlighted</p>
<h2><Icon variant="package" /> Package Details</h2>

<PackageCard pkg={$pkgStore} />
<h2><Icon variant="component" /> Package Components</h2>
<ul>
	{#each $pkgComponentDeployStore as idx}
		<li>{$pkgStore.components[idx].name} will be deployed</li>
	{/each}
</ul>
<div class="actionButtonsContainer">
	<Button href="/initialize/configure" variant="outlined">configure</Button>
	<Button href="/initialize/deploy" variant="flat">deploy</Button>
</div>
