<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { IconButton, currentTheme } from '@ui';

	const toggle = () => {
		currentTheme.update((theme) => {
			return theme === 'dark' ? 'light' : 'dark';
		});
	};
</script>

<IconButton
	on:click={toggle}
	iconContent="dark_mode"
	toggledIconContent="light_mode"
	toggleable
	toggled={$currentTheme === 'dark'}
	iconClass="material-symbols-outlined"
	toggledIconClass="material-symbols-outlined"
/>
