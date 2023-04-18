<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { makeThemeColor, type ThemeColors } from '@defense-unicorns/unicorn-ui';
	export let diameter: string = '20px';
	export let thickness: string = '2px';
	export let color: ThemeColors = 'primary';

	// Need this to force-enable first onload animation to avoid ugly flash on very fast REST calls
</script>

<span
	class="spinner"
	style="--diameter:{diameter};--thickness:{thickness};--color:{makeThemeColor(color)}"
/>

<style lang="scss">
	.spinner {
		width: var(--diameter);
		height: var(--diameter);
		border-radius: 50%;
		border-top: var(--thickness) solid var(--color);
		border-left: var(--thickness) solid var(--color);
		border-bottom: var(--thickness) solid var(--color);
		border-right: var(--thickness) solid transparent;
		animation: spinner 1s linear infinite;
	}

	@keyframes spinner {
		to {
			transform: rotate(360deg);
		}
	}
</style>
