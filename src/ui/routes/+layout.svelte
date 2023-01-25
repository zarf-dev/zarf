<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import '../app.css';
	import 'sanitize.css';
	import '@fontsource/roboto';
	import { Cluster } from '$lib/api';
	import { clusterStore } from '$lib/store';
	import { Header } from '$lib/components';
	import 'material-symbols/';
	import { Theme } from '@ui';
	import { ZarfPalettes } from '$lib/palette';
	import { ZarfTypography } from '$lib/typography';
	import { themeStore } from '$lib/store';

	function getClusterSummary() {
		// Try to get the cluster summary
		Cluster.summary()
			// If success update the store
			.then(clusterStore.set)
			// Otherwise, try again in 250 ms
			.catch((e) => {
				setTimeout(getClusterSummary, 250);
			});
	}

	getClusterSummary();

	$: theme = $themeStore;
</script>

<Header />

<Theme {theme} palettes={ZarfPalettes} typography={ZarfTypography}>
	<main class="mdc-typography">
		<slot />
	</main>
</Theme>
