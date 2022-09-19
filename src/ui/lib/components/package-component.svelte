<script lang="ts">
	import type { ZarfComponent } from '$lib/api-types';
	import { pkgComponentDeployStore } from '$lib/store';
	import { Accordion } from '@defense-unicorns/unicorn-ui';

	export let readOnly: boolean = true;
	export let idx: number;
	export let component: ZarfComponent;

	const toggleComponentDeployment = (list: number[], idx: number) => {
		const enabled = list.includes(idx);
		if (enabled) {
			list = [...list].filter((n) => n !== idx);
		} else {
			list = [...list, idx];
		}
		list.sort();
		pkgComponentDeployStore.set(list);
	};
</script>

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
			<div style="max-width: 250px; white-space: nowrap;overflow: hidden;text-overflow: ellipsis">
				{component.description || ' '}
			</div>
		</div>

		<div>
			{#if readOnly}
				<input disabled={true} checked={$pkgComponentDeployStore.includes(idx)} type="checkbox" />
				<label style="color: #b1b1b1" for={`deploy-component-${idx}`}>Deploy</label>
			{:else}
				<input
					disabled={component.required}
					checked={$pkgComponentDeployStore.includes(idx)}
					type="checkbox"
					id={`deploy-component-${idx}`}
					on:change={() => toggleComponentDeployment($pkgComponentDeployStore, idx)}
				/>
				<label style={component.required ? 'color: #b1b1b1;' : ''} for={`deploy-component-${idx}`}
					>Deploy</label
				>
			{/if}
		</div>
	</div>
	<div slot="content">
		<pre>{JSON.stringify(component, null, 2)}</pre>
	</div>
</Accordion>
