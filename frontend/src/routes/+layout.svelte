<script lang="ts">
	import type { propColor } from '@ui/Button/Button.types';
	import { Button } from '@ui';
	import { afterUpdate } from 'svelte';
	import logo from '@images/zarf-logo.png';
	import 'material-symbols';

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

<section>
	<img alt="Zarf logo" id="logo" src={logo} />

	<Button href="/" variant="outlined" color={getVariant(path, '/')}>Home</Button>
	<Button href="/zarf-state" variant="outlined" color={getVariant(path, '/zarf-state')}
		>View Zarf State</Button
	>
</section>

<br />

<main>
	<slot />
</main>

<style lang="scss">
	#logo {
		display: block;
		margin: 1rem;
		background-repeat: no-repeat;
		background-size: 100% 100%;
	}
</style>
