<script lang="ts">
	import { ViewState } from '@api/K8s';
</script>

{#await ViewState()}
	<h3>Loading the Zarf State from the cluster...</h3>
{:then state}
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
{/await}

<style lang="scss">
	@use '@material/card';
	@use '@material/data-table/data-table';

	@include card.core-styles;
	@include data-table.core-styles;
	@include data-table.theme-baseline;
</style>
