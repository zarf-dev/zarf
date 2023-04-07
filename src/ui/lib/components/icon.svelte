<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts" context="module">
	const zarfSymbols = ['package', 'component', 'secured_layer'];

	type ZarfSymbol = (typeof zarfSymbols)[number];
	export type IconVariant = MaterialSymbol | ZarfSymbol;
</script>

<script lang="ts">
	import { Box } from '@ui';
	import type { MaterialSymbol } from 'material-symbols';

	export let variant: IconVariant;
	export let element = 'span';
	let extraClasses = '';
	export { extraClasses as class };
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
					stroke-width="2"
				/>
			</svg>
		{:else if variant === 'secured_layer'}
			<svg
				width="35"
				height="36"
				viewBox="0 0 35 36"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<mask
					id="mask0_1338_1956"
					style="mask-type:alpha"
					maskUnits="userSpaceOnUse"
					x="0"
					y="0"
					width="35"
					height="36"
				>
					<rect y="0.5" width="35" height="35" fill="#D9D9D9" />
				</mask>
				<g mask="url(#mask0_1338_1956)">
					<path
						d="M9.1337 26.7676C11.3329 29.7941 14.1217 31.7327 17.5002 32.5834C17.5245 32.5834 17.5429 32.5776 17.5556 32.5659C17.5672 32.5533 17.5852 32.547 17.6095 32.547C17.0505 31.9879 16.5522 31.356 16.1147 30.6511C15.6772 29.9463 15.3248 29.2049 15.0575 28.4272C14.1095 27.8438 13.2467 27.1147 12.4689 26.2397C11.6911 25.3647 11.0291 24.4109 10.4827 23.3784C9.9353 22.3449 9.50947 21.257 9.20516 20.1147C8.90183 18.9723 8.75016 17.8299 8.75016 16.6876V9.83341L17.5002 6.55216L26.2502 9.83341V15.1928C26.7606 15.2657 27.2588 15.3809 27.745 15.5384C28.2311 15.6969 28.705 15.8733 29.1668 16.0678V7.79175L17.5002 3.41675L5.8335 7.79175V16.6876C5.8335 20.382 6.93357 23.742 9.1337 26.7676Z"
						fill="white"
					/>
					<path
						d="M23.3332 28.1263L16.0415 23.8695L17.3783 23.1094L23.3332 26.5756L29.288 23.1094L30.6248 23.8695L23.3332 28.1263ZM23.3332 25.0553L16.0415 20.7985L23.3332 16.5417L30.6248 20.7985L23.3332 25.0553ZM23.3332 23.5047L27.9917 20.7985L23.3332 18.0924L18.6746 20.7985L23.3332 23.5047Z"
						fill="white"
					/>
					<path
						d="M23.3332 31.1251L16.0415 26.8683L17.3783 26.1081L23.3332 29.5744L29.288 26.1081L30.6248 26.8683L23.3332 31.1251Z"
						fill="white"
					/>
				</g>
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
