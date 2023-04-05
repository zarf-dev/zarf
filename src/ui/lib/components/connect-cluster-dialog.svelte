<script lang="ts">
	import { Typography, Box, Button, type SSX, Dialog, List, IconButton, DialogActions } from '@ui';
	import { clusterStore } from '$lib/store';
	import ZarfKubeConfigFound from '../../images/png/zarf-kube-config-found.png';
	import ZarfKubeConfigNotFound from '../../images/png/zarf-kube-not-found.png';

	export let toggleDialog: () => void;

	let titleAlt: string;
	let titleImage: string;
	let titleText: string;

	const ssx: SSX = {
		$self: {
			'& .dialog-surface': {
				padding: '24px 16px',
				width: '444px',
				height: '303px',
			},
			'& .dialog-content': {
				width: 'inherit',
				height: 'inherit',
				display: 'flex',
				flexDirection: 'column',
				gap: '8px',
				'& .dialog-header': {
					display: 'flex',
					flexDirection: 'column',
					alignItems: 'center',
					gap: '8px',
					'& h6': {
						marginTop: '10px',
					},
				},
				'& .list': {
					padding: '0px',
					'& > li': {
						display: 'flex',
						alignItems: 'center',
						padding: '0px',
					},
					'& .icon-button': {
						width: '38px !important',
						height: '38px !important',
						'&:hover': {
							background: 'var(--shades-primary-16p)',
						},
						color: 'var(--primary)',
						'& .material-symbols-outlined': {
							fontSize: '17px',
						},
					},
				},
			},
			'& .dialog-actions': {
				gap: '8px',
				padding: '8px 0px',
			},
		},
	};
	$: hasDistro = $clusterStore?.distro;
	$: {
		if (hasDistro) {
			titleText = 'Kubeconfig Found';
			titleImage = ZarfKubeConfigFound;
			titleAlt = 'Zarf, A happy axolotl has found a kubeconfig';
		} else {
			titleText = 'Kubeconfig Not Found';
			titleImage = ZarfKubeConfigNotFound;
			titleAlt = 'Zarf, A sat axolotl was unable to find a kubeconfig';
		}
	}
</script>

<Dialog {ssx} bind:toggleDialog elevation={12}>
	<svelte:fragment slot="content">
		<Box class="dialog-header">
			<img src={titleImage} alt={titleAlt} width="64px" height="64px" />
			<Typography variant="h6">{titleText}</Typography>
		</Box>
		{#if hasDistro}
			<Typography variant="body2" color="text-secondary-on-dark">
				Which cluster would you like to connect to Zarf?
			</Typography>
			<List>
				<Typography variant="body1" element="li" value={$clusterStore?.distro}>
					<IconButton
						toggleable
						toggled
						iconClass="material-symbols-outlined"
						iconContent="radio_button_unchecked"
						iconColor="primary"
						toggledIconClass="material-symbols-outlined"
						toggledIconContent="radio_button_checked"
					/>
					{$clusterStore?.distro}
				</Typography>
			</List>
			<Typography variant="caption" color="text-secondary-on-dark">
				Clusters can be managed in your system Kubconfig file.
			</Typography>
			<DialogActions>
				<Button on:click={toggleDialog} variant="outlined" backgroundColor="grey-300">
					cancel
				</Button>
				<Button
					href="/packages"
					variant="raised"
					backgroundColor="grey-300"
					textColor="text-primary-on-light"
				>
					Connect Cluster
				</Button>
			</DialogActions>
		{:else}
			<Typography variant="caption" color="text-secondary-on-dark">
				Zarf requires access to a cluster. Please install a cluster or login to your cluster
				provider then try connecting to cluster again.
			</Typography>
			<DialogActions>
				<Button
					on:click={toggleDialog}
					variant="raised"
					backgroundColor="grey-300"
					textColor="text-primary-on-light">Close</Button
				>
			</DialogActions>
		{/if}
	</svelte:fragment>
</Dialog>
