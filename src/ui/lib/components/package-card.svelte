<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import type { ZarfPackage } from '$lib/api-types';
	import { Typography } from '@ui';
	export let pkg: ZarfPackage;
</script>

<div class="package-details-card mdc-elevation--z2">
	<div>
		<Typography class="col-content" variant="body1" element="span">{pkg.metadata?.name}</Typography>
	</div>
</div>

<style lang="scss">
	@use '@material/elevation';
	@include elevation.core-styles;

	.package-details-card :global(.mdc-typography--overline) {
		color: var(--mdc-theme-primary-dark);
	}
	.package-details-card {
		width: 100%;
		gap: 1.25rem;
		display: flex;
		padding: 16px;
		text-align: left;
		flex-direction: row;
		background-color: var(--mdc-theme-surface);
	}
</style>
