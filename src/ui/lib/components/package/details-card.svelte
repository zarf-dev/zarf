<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import Divider from '../divider.svelte';
	import type { ZarfPackage } from '$lib/api-types';
	import { Typography } from '@ui';
	export let pkg: ZarfPackage;
</script>

<div class="package-details-card mdc-elevation--z2">
	<div class="package-details-card-col col-1">
		<Typography variant="overline" class="color-primary-dark">Package Type</Typography>
		<Typography class="col-content" variant="body1" element="span">{pkg.kind}</Typography>
	</div>
	<Divider />
	<div class="package-details-card-col col-2">
		<Typography variant="overline" class="color-primary-dark">Metadata</Typography>
		<table>
			<tr>
				<Typography variant="caption" element="td">Name:</Typography>
				<Typography variant="th" element="td">
					{pkg.metadata?.name}
				</Typography>
			</tr>
			<tr>
				<Typography variant="caption" element="td">Description:</Typography>
				<Typography variant="body3" element="td">
					{pkg.metadata?.description}
				</Typography>
			</tr>
		</table>
	</div>
	<Divider />
	<div class="package-details-card-col col-3">
		<Typography variant="overline" element="div" class="color-primary-dark">Build</Typography>
		<div class="col-content">
			<table>
				<tr>
					<Typography variant="caption" element="td">User:</Typography>
					<Typography variant="th" element="td">{pkg.build?.user}</Typography>
					<Typography variant="caption" element="td">Terminal:</Typography>
					<Typography variant="th" element="td">{pkg.build?.terminal}</Typography>
				</tr>
				<tr>
					<Typography variant="caption" element="td">Version:</Typography>
					<Typography variant="th" element="td">{pkg.build?.version}</Typography>
					<Typography variant="caption" element="td">Timestamp:</Typography>
					<Typography variant="th" element="td">{pkg.build?.timestamp}</Typography>
				</tr>
				<tr>
					<Typography variant="caption" element="td">Architecture:</Typography>
					<Typography variant="th" element="td">{pkg.build?.architecture}</Typography>
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
	.package-details-card :global(.mdc-typography--overline) {
		color: var(--mdc-theme-primary-dark);
	}
	.package-details-card {
		width: 100%;
		gap: 1.25rem;
		display: flex;
		padding: 16px;
		text-align: left;
		min-height: 10rem;
		flex-direction: row;
		background-color: var(--mdc-theme-surface);
	}
	.package-details-card-col {
		padding: 0 0.5rem;
	}
	.col-1 {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		min-width: 10%;
	}
	.col-2 {
		min-width: 30%;
	}

	.col-3 {
		min-width: 50%;
	}
	@media (max-width: 1200px) {
		.package-details-card {
			flex-direction: column;
			gap: 1.25rem;
		}
		.col-3,
		.col-2 {
			flex-grow: 1;
		}
	}
</style>
