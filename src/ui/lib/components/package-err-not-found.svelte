<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Dialog, Typography, Button } from '@ui';
	import bigZarf from '@images/zarf-bubbles-right.png';
	export let pkgName: string;
</script>

<Dialog open clickAway={false}>
	<div slot="content" class="dialog-content">
		<!-- @Noxsios TODO: replace w/ sad zarf -->
		<div class="center">
			<img class="zarf-logo" width={60} height={60} src={bigZarf} alt="zarf-logo" />
			<Typography variant="h6">Package Not Found</Typography>
		</div>

		<Typography variant="body2">The current operation encountered the following error:</Typography>
		<Typography variant="code"><code>{pkgName}</code></Typography>
		<div class="actions">
			<Button href="/" color="secondary" variant="flat">Return Home</Button>
		</div>
	</div>
</Dialog>

<style>
	.dialog-content {
		display: flex;
		gap: 1rem;
		flex-direction: column;
		max-width: 444px;
		width: 100%;
	}
	.center {
		display: flex;
		justify-content: center;
		align-items: center;
		flex-direction: column;
		gap: 1rem;
	}
	.actions {
		display: flex;
		width: 100%;
		justify-content: flex-end;
	}
</style>
