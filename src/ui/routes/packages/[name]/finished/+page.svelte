<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Dialog, Typography } from '@ui';
	import zarfLogo from '@images/zarf-bubbles-right.png';
</script>

<svelte:head>
	<title>Finished</title>
</svelte:head>

<Dialog open={true}>
	<div slot="content" style="text-align:center;margin: 0.5rem 2rem 1rem 2rem;">
		<img src={zarfLogo} alt="zarf" width="60px" />
		<Typography style="margin: 1rem 0;" variant="h6">Package Successfully Deployed</Typography>
		<Typography variant="body2">You will be redirected to the deployment details page</Typography>
	</div>
</Dialog>
