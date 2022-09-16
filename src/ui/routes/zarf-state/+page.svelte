<script lang="ts">
	import { Cluster, State } from '$lib/api';
</script>

{#await Cluster.summary() then summary}
	{#if summary.reachable}
		{#if summary.hasZarf}
			{#await State.read() then state}
				<div class="mdc-card">
					<div class="mdc-data-table">
						<div class="mdc-data-table__table-container">
							<table class="mdc-data-table__table" aria-label="Dessert calories">
								<thead>
									<tr class="mdc-data-table__header-row">
										<th class="mdc-data-table__header-cell">Key</th>
										<th class="mdc-data-table__header-cell">Value</th>
									</tr>
								</thead>
								<tbody class="mdc-data-table__content">
									{#each Object.entries(state) as [key, data]}
										<tr class="mdc-data-table__row">
											<th class="mdc-data-table__cell" scope="row">{key}</th>
											<td class="mdc-data-table__cell">{JSON.stringify(data, null, 2)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					</div>
				</div>
			{:catch error}
				<h3>Something went wrong loading the Zarf State for this cluster</h3>
			{/await}
		{:else}
			<h3>Kubernetes cluster found ({summary.distro}), but it has not been initialed by Zarf</h3>
		{/if}
	{:else}
		<p>Could not find a Kubernetes cluster</p>
	{/if}
{/await}

<style lang="scss">
	@use '@material/card';
	@use '@material/data-table/data-table';

	@include card.core-styles;
	@include data-table.core-styles;
	@include data-table.theme-baseline;
</style>
