<script lang="ts">
	import Icon from './icon.svelte';
</script>

<div class="tooltip">
	<Icon variant="info-light" />
	<span class="tooltiptext"><slot /></span>
</div>

<style>
	.tooltip {
		position: relative;
		display: inline-block;
		border-bottom: 1px dotted black;
	}

	.tooltip .tooltiptext {
		visibility: hidden;
		width: 120px;
		background-color: black;
		color: #fff;
		text-align: center;
		border-radius: 6px;
		padding: 5px 0;

		/* Position the tooltip */
		position: absolute;
		z-index: 1;
	}

	.tooltip:hover .tooltiptext {
		visibility: visible;
	}

	/* top */
	.tooltip .tooltiptext {
		width: 120px;
		bottom: 100%;
		left: 50%;
		margin-left: -60px; /* Use half of the width (120/2 = 60), to center the tooltip */
	}

	/* bottom arrow */
	.tooltip .tooltiptext::after {
		content: ' ';
		position: absolute;
		top: 100%; /* At the bottom of the tooltip */
		left: 50%;
		margin-left: -5px;
		border-width: 5px;
		border-style: solid;
		border-color: black transparent transparent transparent;
	}

	/* transition */
	.tooltip .tooltiptext {
		opacity: 0;
		transition: opacity 1s;
	}

	.tooltip:hover .tooltiptext {
		opacity: 1;
	}
</style>
