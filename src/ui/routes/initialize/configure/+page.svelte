<script lang="ts">
	import Icon from '$lib/components/icon.svelte';
	import PackageCard from '$lib/components/package-card.svelte';
	import { pkgStore, pkgComponentDeployStore } from '$lib/store';
	import { Accordion, Button } from '@ui';

	// let componentsToDeploy: number[] = pkgConfig.components
	// 	.filter((c) => c.Required)
	// 	.map((_, idx) => idx);

	const toggleComponentDeployment = (list: number[], idx: number) => {
		const enabled = list.includes(idx);
		if (enabled) {
			list = [...list].filter((n) => n !== idx);
		} else {
			list = [...list, idx];
		}
		pkgComponentDeployStore.set(list);
	};
</script>

<h1>Configure Package Deployment</h1>
<h2><Icon variant="package" /> Package Details</h2>

<PackageCard pkg={$pkgStore} />

<h2><Icon variant="component" /> Package Components</h2>

<div style="width: 100%;gap: 2px; display: flex; flex-direction: column;">
	{#each $pkgStore.components as component, idx}
		<Accordion id={`component-accordion-${idx}`}>
			<div slot="headerContent" class="component-accordion-header">
				<div style="display:flex;width: 60%;justify-content:space-between;">
					<div>
						{component.name}
						{#if component.required}
							<span style="color:gray;">(Required)</span>
						{:else}
							<span style="color:skyblue;">(Optional)</span>
						{/if}
					</div>
					<div
						style="max-width: 250px; white-space: nowrap;overflow: hidden;text-overflow: ellipsis"
					>
						{component.description || ' '}
					</div>
				</div>

				<div>
					<input
						disabled={component.required}
						checked={component.required || $pkgComponentDeployStore.includes(idx)}
						type="checkbox"
						id={`deploy-component-${idx}`}
						on:change={() => toggleComponentDeployment($pkgComponentDeployStore, idx)}
					/>
					<label style={component.required ? 'color: #b1b1b1;' : ''} for={`deploy-component-${idx}`}
						>Deploy</label
					>
				</div>
			</div>
			<div slot="content">
				<pre>{JSON.stringify(component, null, 2)}</pre>
			</div>
		</Accordion>
	{/each}
</div>
<div class="actionButtonsContainer">
	<Button href="/" variant="outlined" shape="square">cancel deployment</Button>
	<Button href="/initialize/review" variant="raised" shape="square">review deployment</Button>
</div>
