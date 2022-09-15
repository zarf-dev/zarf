import type { ZarfState } from './api-types';

const BASE_URL = 'api';
const MAGIC = 'MAGIC';

const headers = {
	Authorization: `Bearer ${MAGIC}`,
	'Content-Type': 'application/json'
};

const getClusterState = async (): Promise<ZarfState> =>
	await fetch(`/${BASE_URL}/cluster/state`, { headers: headers })
		.then((res) => res.json())
		.catch((e) => {
			console.error(e);
			return {};
		});

export { getClusterState };
