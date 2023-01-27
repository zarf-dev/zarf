<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<section class="hero">
	<div class="hero-content">
		<slot />
	</div>
</section>

<style>
	.hero {
		position: relative;
		height: calc(100vh - 8rem);
		display: flex;
		justify-content: center;
		align-items: center;
	}
	.hero-content {
		position: relative;
		text-align: center;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}
</style>
