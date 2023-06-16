<script lang="ts">
	import { type StepProps, Box, type SSX } from '@ui';
	import { onMount } from 'svelte';
	import Convert from 'ansi-to-html';

	const convert = new Convert({
		newline: true,
		colors: {
			0: '#000000',
			1: '#C23621',
			2: '#25BC24',
			3: '#ADAD27',
			4: '#000080',
			5: '#D338D3',
			6: '#33BBC8',
			7: '#CBCCCD',
			8: '#818383',
			9: '#FC391F',
			10: '#31E783',
			11: '#EAEC23',
			12: '#0000E1',
			13: '#F935F8',
			14: '#14F0F0',
			15: '#E9EBEB',
		},
	});
	let termElement: HTMLElement | null;
	let scrollAnchor: Element | null | undefined;

	export let height: string = '688px';
	export let width: string = '751px';
	export const addMessage = (message: string) => {
		let html = convert.toHtml(message);
		html = `<div class="zarf-terminal-line">${html}</div>`;
		scrollAnchor?.insertAdjacentHTML('beforebegin', html);
		scrollAnchor?.scrollIntoView();
	};

	const ssx: SSX = {
		$self: {
			display: 'flex',
			flexDirection: 'column',
			backgroundColor: '#1E1E1E',
			padding: '12px',
			fontSize: '12px',
			overflowY: 'scroll',
			overflowX: 'hidden',
			height: height,
			width: width,
			'& .zarf-terminal-line': {
				whiteSpace: 'pre-wrap',
				wordBreak: 'break-all',
				wordWrap: 'break-word',
				overflowWrap: 'break-word',
			},
		},
	};

	onMount(() => {
		termElement = document.getElementById('terminal');
		scrollAnchor = termElement?.lastElementChild;
	});
</script>

<Box element="pre" {ssx} id="terminal">
	<div class="scroll-anchor" />
</Box>
