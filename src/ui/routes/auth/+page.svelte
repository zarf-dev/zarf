<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { Auth } from '$lib/api';
	import Hero from '$lib/components/hero.svelte';
	import sadDay from '@images/sadness.png';

	let authFailure = false;

	page.subscribe(async ({ url }) => {
		let token = url.searchParams.get('token') || '';
		let next = url.searchParams.get('next');

		if (await Auth.connect(token)) {
			goto(next || '/');
		} else {
			authFailure = true;
		}
	});
</script>

{#if authFailure}
	<Hero>
		<img src={sadDay} alt="Sadness" id="sadness" width="40%" />

		<div class="hero-text">
			<h1 class="hero-title">Could not authenticate!</h1>
			<h2 class="hero-subtitle">
				Please make sure you are using the complete link to connect provided by Zarf.
			</h2>
		</div>
	</Hero>
{/if}
