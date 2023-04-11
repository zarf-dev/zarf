<script lang="ts">
	import { Typography, type TypographyProps } from '@ui';

	type T = $$Generic<EventTarget>;
	export let selected = false;

	interface $$Props extends TypographyProps<T> {
		selected?: boolean;
	}

	$: selectedClass = (selected && 'nav-link-selected') || '';
	$: providedClass = $$restProps.class || '';
</script>

<Typography {...$$restProps} class="nav-link {selectedClass} {providedClass}">
	<slot />
</Typography>

<style lang="scss" global>
	@use '@material/ripple';
	.nav-link {
		width: 100%;
		padding: 0.75rem 1rem;
		@include ripple.surface;
		@include ripple.states;
		@include ripple.radius-unbounded;
	}
	.nav-link::before,
	.nav-link::after {
		border-radius: 0px;
	}
	.nav-link:hover:not(.nav-link-selected) {
		background: var(--action-hover-on-dark);
	}
	.nav-link-selected {
		background: var(--shades-primary-16p);
	}
</style>
