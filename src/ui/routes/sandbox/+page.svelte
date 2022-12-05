<script lang="ts">
	import { onMount } from 'svelte';
	import { Stdout } from '$lib/api';
	import Convert from 'ansi-to-html';
	const convert = new Convert({
		fg: '#FFF',
		bg: '#000'
	});
	let terminal: any;

	onMount(async () => {
		// await Stdout.read().then(text => {
        //     terminal.innerHTML += convert.toHtml(text);
        // });
		const conn = new WebSocket('ws://127.0.0.1:3333/ws/stdout');
		conn.onopen = function(e) {
			terminal.innerHTML += convert.toHtml('Connected to server');
		};
		conn.onmessage = (e) => {
			terminal.innerHTML += convert.toHtml(e.data);
		};
		window.scrollTo(0, document.body.scrollHeight);
	});
</script>

<pre bind:this={terminal} style="background-color: black; color: white;" />
