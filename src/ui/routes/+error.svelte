<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Dialog, Typography } from '@ui';
	import { page } from '$app/stores';
</script>

<Dialog open clickAway={false}>
	<div slot="content" class="dialog-content">
		<Typography variant="code">
			<code>Error: {$page.status} {$page.error?.message}</code>
		</Typography>
	</div>
</Dialog>

<style>
	.dialog-content {
		display: flex;
		gap: 1rem;
		flex-direction: column;
		align-items: center;
	}
</style>
