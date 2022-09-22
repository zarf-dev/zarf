import { goto } from '$app/navigation';
import type { LoadEvent } from '@sveltejs/kit';

// @todo: this is sort of hacky and gross rn....
export async function load({ url }: LoadEvent) {
	let token = url.searchParams.get('token');
	let next = url.searchParams.get('next');

	if (!token) {
		return {
			status: 400,
			error: new Error('Missing token')
		};
	}
	window.sessionStorage.setItem('token', token);
	goto(next || '/');
}
