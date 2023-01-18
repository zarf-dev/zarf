<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Box } from '@ui';
	import type { MaterialSymbol } from 'material-symbols';
	import { themeStore } from '$lib/store';

	const zarfSymbols = ['package', 'component'];

	type ZarfSymbol = (typeof zarfSymbols)[number];

	export let variant: MaterialSymbol | ZarfSymbol;
	export let element = 'span';
	let extraClasses = '';
	export { extraClasses as class };

	$: stroke = $themeStore === 'dark' ? 'var(--mdc-theme-on-background)' : '#0D133D';
</script>

{#if zarfSymbols.includes(variant)}
	<Box {element} class={`${extraClasses || ''}`}>
		{#if variant === 'component'}
			<svg
				width="36"
				height="33"
				viewBox="0 0 36 33"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					d="M26.0213 4.10413L18.0005 7.02079M26.0213 4.10413L18.0005 1.91663L9.97965 4.10413M26.0213 4.10413V15.0416M18.0005 7.02079L9.97965 4.10413M18.0005 7.02079V17.9583M9.97965 4.10413V15.0416M26.0213 15.0416L18.0005 17.9583M26.0213 15.0416L34.0422 17.9583M18.0005 17.9583L9.97965 15.0416M18.0005 17.9583V27.4375M18.0005 17.9583L26.0213 20.875M18.0005 17.9583L9.25049 20.875M9.97965 15.0416L1.22965 17.9583M18.0005 27.4375L26.0213 31.0833M18.0005 27.4375L9.25049 31.0833M26.0213 31.0833V20.875M26.0213 31.0833L34.0422 26.7083V17.9583M26.0213 20.875L34.0422 17.9583M9.25049 31.0833L1.22965 26.7083V17.9583M9.25049 31.0833V20.875M1.22965 17.9583L9.25049 20.875"
					{stroke}
					stroke-width="2"
				/>
			</svg>
		{:else if variant === 'package'}
			<svg
				width="35"
				height="28"
				viewBox="0 0 35 28"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					d="M17.4236 5.62058L13.6364 1.83337L4.16834 3.96368L7.95554 8.46099M17.4236 5.62058L21.6842 1.83337L31.3889 4.67378L27.1283 8.46099M17.4236 5.62058L7.95554 8.46099M17.4236 5.62058L27.1283 8.46099M27.1283 8.46099L32.0833 12.25L22.6042 16.4167L17.2581 12.25M27.1283 8.46099L17.2581 12.25M17.2581 12.25L12.3958 16.4167L2.91667 11.5556L7.95554 8.46099M17.2581 12.25L7.95554 8.46099M8.42894 14.618V22.663L17.6603 26.2135L27.1283 22.4263V14.618"
					{stroke}
					stroke-width="2"
				/>
			</svg>
		{/if}
	</Box>
{:else}
	<Box {element} class={`material-symbols-outlined ${extraClasses || ''}`}>
		{variant}
	</Box>
{/if}
