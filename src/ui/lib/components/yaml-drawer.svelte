<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Drawer, IconButton, Typography, type SSX } from '@defense-unicorns/unicorn-ui';
	import { YamlCode } from '.';
	export let code: any = undefined;
	export let title = 'Package YAML';
	export let drawerOpen = false;
	export let toggleDrawer = () => {
		drawerOpen = !drawerOpen;
	};

	const ssx: SSX = {
		$self: {
			minWidth: '80ch',
			width: 'max-content',
			maxWidth: '120ch',
			height: 'calc(100% - 3.5rem)',
			marginTop: '3.5rem',
			'& .mdc-drawer__content': {
				flexDirection: 'column',
				flexGrow: 1,
				overflow: 'hidden',
			},
			'& .yaml-drawer-header': {
				display: 'flex',
				justifyContent: 'space-between',
				height: '48px',
				alignItems: 'center',
				padding: '0px 16px',
			},
			'& .yaml-drawer-code': {
				paddingBottom: '48px',
				height: 'calc(100% - 48px)',
				overflow: 'auto',
				'& > pre': {
					height: '100%',
					padding: '16px',
				},
			},
		},
	};
</script>

<Drawer {ssx} anchor="right" modal open={drawerOpen} onClose={toggleDrawer}>
	<div class="yaml-drawer-header">
		<div>
			<slot name="title">
				<Typography variant="subtitle">{title}</Typography>
			</slot>
		</div>
		<div>
			<slot name="actions">
				<IconButton
					iconContent="close"
					iconClass="material-symbols-outlined"
					on:click={toggleDrawer}
				/>
			</slot>
		</div>
	</div>
	<div class="yaml-drawer-code">
		<YamlCode {code} />
	</div>
</Drawer>
