/* eslint-disable @typescript-eslint/no-explicit-any */
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
import { fetchEventSource, type EventSourceMessage } from '@microsoft/fetch-event-source';
const BASE_URL = '/api';

interface APIRequest<T> {
	path: string;
	method: string;
	body?: T;
}

type ResponseType = 'json' | 'boolean' | 'text';

export interface EventParams {
	onopen?: (response: Response) => Promise<void>;
	onmessage?: (ev: EventSourceMessage) => void;
	onclose?: () => void;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	onerror?: (err: any) => number | null | undefined | void;
	openWhenHidden?: boolean;
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

	eventStream<T>(path: string, eventParams: EventParams): AbortController {
		return this.connect<T>({ path, method: 'GET' }, eventParams);
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
		return this.request<boolean>({ path, method: 'HEAD' }, 'boolean');
	}

	// Performs a DELETE request to the given path, and returns response.ok if successful
	async del(path: string) {
		try {
			const response = await this.request<boolean>({ path, method: 'DELETE' }, 'boolean');
			return response;
			// eslint-disable-next-line @typescript-eslint/no-explicit-any
		} catch (e: any) {
			throw new Error(e.message);
		}
	}

	private invalidateAuth() {
		sessionStorage.removeItem('token');
		if (location.pathname !== '/auth') {
			location.pathname = '/auth';
		}
	}

	// Private handler for establishing event source connections.
	private connect<T>(req: APIRequest<T>, eventParams: EventParams): AbortController {
		const url = BASE_URL + req.path;
		const payload: RequestInit = { method: req.method, headers };
		const token = headers.get('Authorization');
		if (!token) {
			throw new Error('Not authenticated yet');
		}

		const abortCtlr = new AbortController();
		fetchEventSource(url, {
			method: req.method,
			headers: { Authorization: token },
			...eventParams,
			body: payload.body,
			signal: abortCtlr.signal,
		});
		return abortCtlr;
	}

	// Private wrapper for handling the request/response cycle.
	private async request<T>(req: APIRequest<T>, responseType: ResponseType = 'json'): Promise<T> {
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

			switch (responseType) {
				case 'boolean':
					return response.ok as T;
				case 'text':
					return (await response.text()) as T;
				default:
					return (await response.json()) as T;
			}

			// Return the response as the expected type
		} catch (e) {
			// Something went really wrong--abort the request.
			console.error(e);
			return Promise.reject(e);
		}
	}
}
