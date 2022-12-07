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

	/* right */
	.tooltip .tooltiptext {
		top: -5px;
		left: 105%;
	}

	/* right arrow */
	.tooltip .tooltiptext::after {
		content: ' ';
		position: absolute;
		top: 50%;
		left: 100%; /* To the right of the tooltip */
		margin-top: -5px;
		border-width: 5px;
		border-style: solid;
		border-color: transparent transparent transparent black;
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
