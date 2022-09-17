<script>
	let modal;
	export let open;
	if (open === false) {
		modal.style.display = 'none';
	}
	window.onclick = function (event) {
		if (event.target == modal && open == false) {
			modal.style.display = 'none';
		}
	};
</script>

<div class="modal" bind:this={modal} on:closeModal>
	<div class="modal-container mdc-elevation--z8">
		<slot />
	</div>
</div>

<style lang="scss">
	@use '@material/elevation';
	@include elevation.core-styles;

	.modal {
		display: block;
		position: fixed; /* Stay in place */
		z-index: 1; /* Sit on top */
		left: 0;
		top: 0;
		width: 100%; /* Full width */
		height: 100%; /* Full height */
		overflow: auto; /* Enable scroll if needed */
		background-color: rgb(0, 0, 0); /* Fallback color */
		background-color: rgba(0, 0, 0, 0.7); /* Black w/ opacity */
	}

	/* Modal Content/Box */
	.modal-container {
		background-color: #fefefe;
		margin: 15% auto; /* 15% from the top and centered */
		padding: 20px;
		width: fit-content;
		max-width: 60%;
		border-radius: 0.25rem;
	}
</style>
