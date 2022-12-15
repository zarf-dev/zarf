<script lang="ts">
	import { Dialog, Typography, Button } from '@ui';
	import bigZarf from '@images/zarf-bubbles-right.png';
	export let pkgName: string;
</script>

<Dialog open clickAway={false}>
	<div slot="content" class="dialog-content">
		<!-- @Noxsios TODO: replace w/ sad zarf -->
		<img class="zarf-logo" width={60} height={60} src={bigZarf} alt="zarf-logo" />
		<Typography variant="h6">Package Not Found</Typography>
		<Typography variant="body2">
			Make sure the following package is in the current working directory:
		</Typography>
		<!-- @Noxsios TODO: replace w/ inline code implementation -->
		<Typography variant="code"><code>{pkgName}</code></Typography>
		<div class="actions">
			<Button href="/" color="secondary" variant="flat"
				>Return Home</Button
			>
		</div>
	</div>
</Dialog>

<style>
	.dialog-content {
		display: flex;
		gap: 1rem;
		flex-direction: column;
		align-items: center;
	}
	.actions {
		display: flex;
		width: 100%;
		justify-content: flex-end;
	}
</style>
