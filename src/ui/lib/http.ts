// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
const BASE_URL = '/api';

interface APIRequest<T> {
	path: string;
	method: string;
	body?: T;
}

// Store this outside of the class vs private since private isn't real in JS.
const headers = new Headers({
	'Content-Type': 'application/json',
});

export class HTTP {
	constructor() {
		const token = sessionStorage.getItem('token') || '';
		if (!token) {
			this.invalidateAuth();
		} else {
			headers.append('Authorization', token);
		}
	}

	// Updates the internal token used for authentication.
	updateToken(token: string) {
		sessionStorage.setItem('token', token);
		headers.set('Authorization', token);
	}

	deployStream<T>(path: string): EventSource {
		return this.connect<T>({ path, method: 'GET' });
	}

	// Perform a GET request to the given path, and return the response as JSON.
	get<T>(path: string) {
		return this.request<T>({ path, method: 'GET' });
	}

	// Performs a POST request to the given path, and returns the response as JSON.
	post<T>(path: string, body: any) {
		return this.request<T>({ path, method: 'POST', body });
	}

	// Performs a PUT request to the given path, and returns the response as JSON.
	put<T>(path: string, body: any) {
		return this.request<T>({ path, method: 'PUT', body });
	}

	// Performs a PATCH request to the given path, and returns the response as JSON.
	patch<T>(path: string, body: any) {
		return this.request<T>({ path, method: 'PATCH', body });
	}

	head(path: string) {
		return this.request<boolean>({ path, method: 'HEAD' });
	}

	// Performs a DELETE request to the given path, and returns the response as JSON.
	async del(path: string) {
		try {
			const response = await this.request<boolean>({ path, method: 'DELETE' });
			return response;
		} catch (e) {
			return false;
		}
	}

	private invalidateAuth() {
		sessionStorage.removeItem('token');
		if (location.pathname !== '/auth') {
			location.pathname = '/auth';
		}
	}

	// Private handler for establishing event source connections.
	private connect<T>(req: APIRequest<T>): EventSource {
		const url = BASE_URL + req.path;
		if (!headers.get('Authorization')) {
			throw new Error('Not authenticated yet');
		}
		const eventSource = new EventSource(`${url}?auth=${headers.get('Authorization')}`);
		return eventSource;
	}

	// Private wrapper for handling the request/response cycle.
	private async request<T>(req: APIRequest<T>): Promise<T> {
		const url = BASE_URL + req.path;
		const payload: RequestInit = { method: req.method, headers };

		if (!headers.get('Authorization')) {
			throw new Error('Not authenticated yet');
		}

		try {
			// Add the body if it exists
			if (req.body) {
				payload.body = JSON.stringify(req.body);
			}

			// Actually make the request
			const response = await fetch(url, payload);

			// Head just returns response.ok
			if (req.method === 'HEAD') {
				return response.ok as T;
			}

			// If the response is not OK, throw an error.
			if (!response.ok) {
				// all API errors should be 500s w/ a text body
				const errMessage = await response.text();
				throw new Error(errMessage);
			}

			// Return the response as the expected type
			return (await response.json()) as T;
		} catch (e) {
			// Something went really wrong--abort the request.
			console.error(e);
			return Promise.reject(e);
		}
	}
}

function makeWriteableEventStream(eventTarget: EventTarget) {
	return new WritableStream({
		start() {
			eventTarget.dispatchEvent(new Event('start'));
		},
		write(message) {
			console.log(JSON.stringify(message));
			eventTarget.dispatchEvent(new MessageEvent('message', { data: message.data }));
		},
		close() {
			eventTarget.dispatchEvent(new CloseEvent('close'));
		},
		abort(reason) {
			eventTarget.dispatchEvent(new CloseEvent('abort', { reason }));
		},
	});
}
