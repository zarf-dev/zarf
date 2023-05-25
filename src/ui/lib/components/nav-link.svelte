<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Typography, type TypographyProps } from '@ui';

	type T = $$Generic<EventTarget>;
	export let selected = false;

	interface $$Props extends TypographyProps<T> {
		selected?: boolean;
	}

	$: selectedClass = (selected && 'nav-link-selected') || '';
	$: providedClass = $$restProps.class || '';
</script>

<Typography {...$$restProps} class="nav-link ripple {selectedClass} {providedClass}">
	<slot />
</Typography>

<style lang="scss" global>
	.nav-link {
		width: 100%;
		padding: 0.75rem 1rem;
	}
	.nav-link-selected {
		background: var(--shades-primary-16p);
	}
</style>
