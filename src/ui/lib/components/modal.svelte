<script>
	// Get the modal
	let modal;

	// When the user clicks anywhere outside of the modal, close it
	window.onclick = function (event) {
		if (event.target == modal) {
			modal.style.display = 'none';
		}
	};
</script>

<div class="modal" bind:this={modal}>
	<div class="modal-content mdc-elevation--z8">
		<div class="modal-body">
			<p>Some text in the Modal Body</p>
			<p>Some other text...</p>
		</div>
		<div class="modal-footer">
			<h3>Modal Footer</h3>
		</div>
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
		background-color: rgba(0, 0, 0, 0.4); /* Black w/ opacity */
	}

	/* Modal Content/Box */
	.modal-content {
		background-color: #fefefe;
		margin: 15% auto; /* 15% from the top and centered */
		padding: 20px;
		width: fit-content;
		max-width: 60%;
		border-radius: 0.25rem;
	}
</style>
