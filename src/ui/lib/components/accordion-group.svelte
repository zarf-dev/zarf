<!-- 
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
 -->
<div class="accordion-group"><slot /></div>

<style>
	.accordion-group {
		width: 100%;
		display: flex;
		flex-direction: column;
		border-radius: 4px 4px 0 0;
	}
	.accordion-group > :global(:first-child) {
		border-radius: 4px 4px 0 0;
	}
	.accordion-group > :global(:last-child) {
		border-radius: 0 0 4px 4px;
	}

	.accordion-group :global(.accordion) {
		margin: 0;
	}
	.accordion-group :global(.accordion:after) {
		content: '';
		background-color: rgba(0, 0, 0, 0.12);
		height: 1px;
	}
</style>
