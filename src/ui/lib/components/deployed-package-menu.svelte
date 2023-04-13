<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { Kind, type DeployedPackage } from '$lib/api-types';
	import {
		IconButton,
		ListItem,
		ListItemAdornment,
		Menu,
		Dialog,
		Typography,
		Button,
		type SSX,
	} from '@ui';
	import { current_component } from 'svelte/internal';
	import bigZarf from '@images/zarf-bubbles-right.png';
	import { Packages } from '$lib/api';

	export let pkg: DeployedPackage;
	let anchorRef: HTMLButtonElement;
	let updateLink: string = '';
	let toggled: boolean = false;
	let dialogOpen: boolean = false;
	let dialogTop: string = '';
	let dialogBottom: string = '';
	let toggleDialog: () => void;

	const menuSSX: SSX = {
		$self: {
			'& .list-item-adornment': { color: 'var(--text-secondary-on-dark)' },
			'& .divider': { height: '1px', boxShadow: 'inset 0px -1px 0px rgba(255,255,255,0.12)' },
		},
	};

	function handleClick(event: any) {
		if (anchorRef && !anchorRef.contains(event.target) && !event.defaultPrevented) {
			if (anchorRef !== event.target) {
				toggled = false;
			}
		}
	}

	async function removePkg(): Promise<void> {
		Packages.remove(pkg.name).catch((err) => {
			dialogTop = `Failed to remove package ${pkg.name}`;
			dialogBottom = err.message;
		});
	}

	$: {
		if (pkg.data.kind === Kind.ZarfInitConfig) {
			updateLink = '/packages?init=true';
		} else {
			updateLink = '/packages';
		}
	}
</script>

<svelte:window on:click={handleClick} />
<IconButton
	eventComponent={current_component}
	bind:ref={anchorRef}
	toggleable
	bind:toggled
	on:click={() => (toggled = !toggled)}
	iconClass="material-symbols-outlined"
	iconContent="more_vert"
/>
<Menu ssx={menuSSX} bind:anchorRef open={toggled} anchorOrigin="bottom-end">
	<ListItem text="Update Package..." on:click={() => goto(updateLink)}>
		<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined"
			>cached</ListItemAdornment
		>
	</ListItem>
	<div class="divider" />
	<ListItem text="Remove..." on:click={removePkg}>
		<ListItemAdornment slot="leading-adornment" class="material-symbols-outlined">
			delete
		</ListItemAdornment>
	</ListItem>
</Menu>
<Dialog bind:open={dialogOpen} bind:toggleDialog>
	<section class="success-dialog" slot="content">
		<img class="zarf-logo" src={bigZarf} alt="zarf-logo" />
		<Typography variant="h6" color="on-background">{dialogTop}</Typography>
		<Typography variant="body2">{dialogBottom}</Typography>
	</section>
	<section slot="actions">
		<Button
			on:click={toggleDialog}
			variant="raised"
			backgroundColor="grey-300"
			textColor="text-primary-on-light">Close</Button
		>
	</section>
</Dialog>

<style>
	.success-dialog {
		display: flex;
		padding: 24px 16px;
		width: 444px;
		height: 220.67px;
		text-align: center;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1rem;
	}
	.zarf-logo {
		width: 64px;
		height: 62.67px;
	}
</style>
