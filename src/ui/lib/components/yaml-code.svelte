<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import Prism from 'prismjs';
	import 'prismjs/components/prism-yaml';
	import 'prismjs/themes/prism-okaidia.css';
	import { stringify } from 'yaml';

	import type { ZarfComponent } from '$lib/api-types';
	import { onMount } from 'svelte';

	export let component: ZarfComponent;

	const yaml = stringify(component);

	onMount(() => {
		Prism.highlightAll();
	});
</script>

<!--Tag formatting matters for pre tags. New line in tags will change the spacing/alignment of the pre-content -->
<pre><code class="language-yaml">{yaml}</code></pre>

<style lang="scss">
	pre {
		border-radius: unset;
		margin: 0;
		padding: 24px;
	}
</style>
