import type { ZarfState } from './api-types';

const request = async <T>(
	url: string,
	method = 'GET',
	body: BodyInit | null | undefined = undefined
): Promise<T | Error> => {
	const _base = '/api/';
	const _magic = 'MAGIC';
	const headers = {
		Authorization: `Bearer ${_magic}`,
		'Content-Type': 'application/json'
	};
	return fetch(_base + url, { headers, method, body })
		.then((res) => res.json())
		.then((data) => {
			console.debug('fetch -->', url);
			return data as T;
		})
		.catch((e) => {
			console.error(e);
			throw e;
		});
};

const Cluster = {
	getState: () => request<ZarfState>('cluster/state'),
	setState: (body: BodyInit | null | undefined) => request<ZarfState>('cluster/state', 'PUT', body)
};

export { Cluster };
