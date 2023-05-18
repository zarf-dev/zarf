<script lang="ts">
	import { Typography, type TypographyProps } from '@ui';

	type T = $$Generic<EventTarget>;

	export let text: string;

	export function copyToClipboard() {
		copied = true;
		navigator.clipboard.writeText(text);
		setTimeout(() => {
			copied = false;
		}, 1000);
	}

	let copied = false;

	interface $$Props extends TypographyProps<T> {
		text: string;
	}

	$: computedClass = `copy-to-clipboard ripple material-symbols-outlined ${
		$$restProps.class || ''
	}`;
	$: content = (copied && 'library_add_check') || 'content_copy';
</script>

<Typography
	{...$$restProps}
	title="Copy SBOM path to clipboard"
	role="button"
	on:click={copyToClipboard}
	class={computedClass}
>
	{content}
</Typography>

<style>
	:global(.copy-to-clipboard) {
		vertical-align: middle;
		border: 1px solid transparent;
		border-radius: 100%;
		padding: 4px;
		line-height: unset !important;
	}
</style>
