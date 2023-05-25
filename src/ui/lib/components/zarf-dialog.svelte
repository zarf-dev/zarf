<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Typography, Box, type SSX, Dialog, DialogActions } from '@ui';
	import ZarfHappy from '../../images/png/zarf-kube-config-found.png';
	import ZarfSad from '../../images/png/zarf-kube-not-found.png';
	export let open: boolean = false;
	export let toggleDialog: () => void;
	export let happyZarf = true;
	export let titleText = 'Zarf Dialog';
	export let ssx: SSX = {};
	export let clickAway = true;
	export let zarfAlt = '';
	let titleAltPrefix: string;
	let titleImage: string;

	$: {
		if (happyZarf) {
			titleImage = ZarfHappy;
			titleAltPrefix = 'Zarf, A happy axolotl.';
		} else {
			titleImage = ZarfSad;
			titleAltPrefix = 'Zarf, A sad axolotl.';
		}
	}
</script>

<Dialog {clickAway} bind:open class="zarf-dialog" {ssx} bind:toggleDialog elevation={12}>
	<svelte:fragment slot="content">
		<Box class="dialog-header">
			<img src={titleImage} alt="{titleAltPrefix} {zarfAlt}" width="64px" height="64px" />
			<Typography variant="h6">{titleText}</Typography>
		</Box>
		<slot />
		<DialogActions>
			<slot name="actions" />
		</DialogActions>
	</svelte:fragment>
</Dialog>

<style global>
	.zarf-dialog {
		width: 444px;
	}
	.zarf-dialog .dialog-surface {
		padding: 24px 16px;
		width: 444px;
		height: 303px;
	}
	.zarf-dialog .dialog-content {
		width: inherit;
		height: inherit;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.zarf-dialog .dialog-content .dialog-header {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
	}
	.zarf-dialog .dialog-content .dialog-header h6 {
		margin-top: 10px;
	}

	.zarf-dialog .dialog-actions {
		gap: 8px;
		padding: 8px 0px;
	}
</style>
