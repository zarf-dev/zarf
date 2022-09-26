<script lang="ts">
	import type { ZarfPackage } from '$lib/api-types';
	export let pkg: ZarfPackage;
</script>

<div class="card mdc-elevation--z2">
	<div class="card-col col-1">
		<div class="col-header">Package Type</div>
		<div class="col-content">{pkg.kind}</div>
	</div>
	<div class="card-col col-2">
		<div class="col-header">Meta data</div>
		<table>
			<tr>
				<td>Name:</td>
				<td>{pkg.metadata?.name}</td>
			</tr>
			<tr>
				<td>Description:</td>
				<td>{pkg.metadata?.description}</td>
			</tr>
		</table>
	</div>
	<div class="card-col col-3">
		<div class="col-header">Build</div>
		<div class="col-content">
			<table>
				<tr>
					<td>User:</td>
					<td>{pkg.build?.user}</td>
					<td>Terminal:</td>
					<td>{pkg.build?.terminal}</td>
				</tr>
				<tr>
					<td>Version:</td>
					<td>{pkg.build?.version}</td>
					<td>Timestamp:</td>
					<td>{pkg.build?.timestamp}</td>
				</tr>
				<tr>
					<td>Architecture:</td>
					<td>{pkg.build?.architecture}</td>
				</tr>
			</table>
		</div>
	</div>
</div>

<style lang="scss">
	@use '@material/elevation';
	@include elevation.core-styles;

	table {
		border-collapse: collapse;
		width: 100%;
	}
	td {
		font-size: 14px;
		line-height: 20px;
		letter-spacing: 0.17px;
	}
	td:nth-child(odd) {
		text-align: left;
		color: #00000099;
		font-size: 12px;
		line-height: 166%;
		letter-spacing: 0.4px;
	}

	td:nth-child(even) {
		font-weight: bold;
		vertical-align: text-bottom;
	}

	.card {
		display: flex;
		flex-direction: row;
		border-radius: 0.25rem;
		padding-left: 0.5rem;
		padding-right: 0.5rem;
		padding-top: 0.375rem;
		padding-bottom: 0.375rem;
	}
	.col-header {
		color: #00acac;
		text-transform: uppercase;
		font-size: 12px;
		line-height: 32px;
		letter-spacing: 1px;
	}
	.card-col {
		padding: 0.5rem 0;
	}
	.col-1 {
		flex-grow: 0.2;
	}
	.col-2 {
		flex-grow: 0.6;
		max-width: 350px;
		border-left: 1px solid #0000001f;
		padding-left: 1.25rem;
	}
	.col-3 {
		flex-grow: 1;
		border-left: 1px solid #0000001f;
		padding-left: 1.25rem;
	}
</style>
