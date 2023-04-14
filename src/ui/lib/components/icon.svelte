<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts" context="module">
	const zarfSymbols = ['package', 'component', 'secured_layer', 'cubes'];

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
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<mask
					id="mask0_943_45675"
					style="mask-type:alpha"
					maskUnits="userSpaceOnUse"
					x="0"
					y="0"
					width="24"
					height="24"
				>
					<rect width="24" height="24" fill="#D9D9D9" />
				</mask>
				<g mask="url(#mask0_943_45675)">
					<path
						d="M6.263 18.012C7.771 20.0873 9.68333 21.4167 12 22C12.0167 22 12.0293 21.996 12.038 21.988C12.046 21.9793 12.0583 21.975 12.075 21.975C11.6917 21.5917 11.35 21.1583 11.05 20.675C10.75 20.1917 10.5083 19.6833 10.325 19.15C9.675 18.75 9.08333 18.25 8.55 17.65C8.01667 17.05 7.56267 16.396 7.188 15.688C6.81267 14.9793 6.52067 14.2333 6.312 13.45C6.104 12.6667 6 11.8833 6 11.1V6.4L12 4.15L18 6.4V10.075C18.35 10.125 18.6917 10.204 19.025 10.312C19.3583 10.4207 19.6833 10.5417 20 10.675V5L12 2L4 5V11.1C4 13.6333 4.75433 15.9373 6.263 18.012Z"
						fill="white"
					/>
					<path
						d="M16 18.9437L11 16.0248L11.9167 15.5035L16 17.8804L20.0833 15.5035L21 16.0248L16 18.9437ZM16 16.8379L11 13.9189L16 11L21 13.9189L16 16.8379ZM16 15.7746L19.1944 13.9189L16 12.0633L12.8056 13.9189L16 15.7746Z"
						fill="white"
					/>
					<path
						d="M16 21L11 18.0811L11.9167 17.5598L16 19.9367L20.0833 17.5598L21 18.0811L16 21Z"
						fill="white"
					/>
				</g>
			</svg>
		{:else if variant === 'package'}
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<mask
					id="mask0_31_17976"
					style="mask-type:alpha"
					maskUnits="userSpaceOnUse"
					x="0"
					y="0"
					width="24"
					height="24"
				>
					<rect width="24" height="24" fill="#D9D9D9" />
				</mask>
				<g mask="url(#mask0_31_17976)">
					<path
						fill-rule="evenodd"
						clip-rule="evenodd"
						d="M14.6813 3.16357L23.0344 5.60841L19.7801 8.50119L23.4643 11.3185L19.3522 13.126V18.6287L12.1153 21.5235L5.02984 18.7983V13.0633L0.4711 10.7255L4.32043 8.36146L1.48972 4.99999L9.58893 3.17766L11.9779 5.56659L14.6813 3.16357ZM11.9502 7.37921L7.70721 8.65211L11.8423 10.3364L16.2598 8.64055L11.9502 7.37921ZM18.4797 9.3951L13.3422 11.3673L15.608 13.1333L20.5357 10.9673L18.4797 9.3951ZM10.4123 11.3736L5.52407 9.38255L3.5289 10.6079L8.39426 13.1029L10.4123 11.3736ZM5.71158 7.68475L4.22685 5.92163L9.11234 4.8224L10.5294 6.23941L5.71158 7.68475ZM11.8567 12.1113L8.60574 14.8971L6.52984 13.8326V17.7681L12.1045 19.9122L17.8522 17.6131V13.7853L15.392 14.8668L11.8567 12.1113ZM18.4144 7.70822L13.4582 6.25765L15.057 4.83648L20.0132 6.28706L18.4144 7.70822Z"
						fill="white"
					/>
				</g>
			</svg>
		{:else if variant === 'cubes'}
			<svg
				width="24"
				height="24"
				viewBox="0 0 24 24"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<mask
					id="mask0_31_17973"
					style="mask-type:alpha"
					maskUnits="userSpaceOnUse"
					x="0"
					y="0"
					width="24"
					height="24"
				>
					<rect width="24" height="24" fill="#D9D9D9" />
				</mask>
				<g mask="url(#mask0_31_17973)">
					<path
						fill-rule="evenodd"
						clip-rule="evenodd"
						d="M12 1.22266L18.25 2.9272V10.4747L23.75 12.4747V19.4453L17.5305 22.8377L11.987 20.318L5.95654 22.8307L-0.25 19.4453V12.4595L5.75 10.4595V2.9272L12 1.22266ZM6.48927 11.7942L2.77932 13.0308L6.01073 14.2059L9.72068 12.9693L6.48927 11.7942ZM11.25 14.0406L6.75 15.5406V20.875L11.25 19V14.0406ZM12.75 19.0171L16.75 20.8353V15.5254L12.75 14.0708V19.0171ZM14.1946 13L17.5 14.202L20.8054 13L17.5 11.7981L14.1946 13ZM16.75 10.4747V4.57082L12.75 6.02537V11.9293L16.75 10.4747ZM11.25 11.9293V6.02537L7.25 4.57082V10.4747L11.25 11.9293ZM8.97569 3.60225L12 4.702L15.0243 3.60225L12 2.77744L8.97569 3.60225ZM22.25 14.0708L18.25 15.5254V20.7366L22.25 18.5548V14.0708ZM5.25 20.7366V15.5254L1.25 14.0708V18.5548L5.25 20.7366Z"
						fill="white"
					/>
				</g>
			</svg>
		{/if}
	</Box>
{:else}
	<Box {element} class={`material-symbols-outlined ${extraClasses || ''}`}>
		{variant}
	</Box>
{/if}
