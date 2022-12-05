<script lang="ts">
	import Modal from '$lib/components/modal.svelte';
	import { Button, Typography } from '@defense-unicorns/unicorn-ui';
	import bigZarf from '@images/zarf-bubbles-right.png';
	export let open = true;
	export let title = 'Dialog Message';
	export let caption = 'Dialog secondary message';
	export let secondaryActionLabel = 'Secondary';
	export let primaryActionLabel = 'Primary';
	export let secondaryAction: EventListener;
	export let primaryAction: EventListener;
</script>

<Modal {open}>
	<div class="action-dialog-content">
		<img class="zarf-logo" src={bigZarf} alt="zarf-logo" />
		<Typography variant="h6">{title}</Typography>
		<Typography variant="body2">{caption}</Typography>
		<div class="action-dialog-actions">
			<Button on:click={secondaryAction} variant="outlined" color="secondary" class="dialog-button"
				>{secondaryActionLabel}</Button
			>
			<Button on:click={primaryAction} variant="raised" color="secondary" class="dialog-button"
				>{primaryActionLabel}</Button
			>
		</div>
	</div>
</Modal>

<style>
	.zarf-logo {
		width: 60px;
		height: 60px;
		margin: 0.5rem;
	}
	.action-dialog-content {
		display: flex;
		flex: 1;
		flex-direction: column;
		align-items: center;
		min-width: 25em;
		gap: 0.5rem;
        margin-top: 0.5rem;
        margin-bottom: 0.5rem;
	}
	.action-dialog-actions {
		margin-top: 0.5rem;
		display: flex;
		justify-content: flex-end;
		width: 100%;
		gap: 0.5rem;
	}
</style>
