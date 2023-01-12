<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<script lang="ts">
	import { Modal } from '$lib/components';
	import zarfLogo from '@images/zarf-bubbles-right.png';
</script>

<Modal open={true}>
	<div style="text-align:center;">
		<img src={zarfLogo} alt="zarf" width="60px" />
		<h3 style="font-size: 20px; color: #000000DE;">Package Successfully Deployed</h3>
		<p style="color: gray; font-size: 14px;">
			You will be redirected to the deployment details page
		</p>
	</div>
</Modal>
