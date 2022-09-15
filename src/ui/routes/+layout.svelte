<script lang="ts">
	import type { propColor } from '@ui/Button/Button.types';
	import { Button } from '@ui';
	import { afterUpdate } from 'svelte';
	import 'material-symbols';
	import Header from '$lib/components/header.svelte';

	let path = '';

	// Lifecycle
	afterUpdate(() => {
		path = window.location.pathname;
	});

	// Functions
	function getVariant(pathname: string, currentPath: string): propColor {
		return pathname === currentPath ? 'secondary' : 'primary';
	}
</script>

<Header />

<!-- <section>
	<Button href="/" variant="outlined" color={getVariant(path, '/')}>Home</Button>
	<Button href="/zarf-state" variant="outlined" color={getVariant(path, '/zarf-state')}
		>View Zarf State</Button
	>
</section> -->

<main>
	<slot />
</main>
