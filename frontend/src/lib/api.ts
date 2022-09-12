import type { ZarfState } from "./api-types";

const MAGIC = "MAGIC";

const Cluster = {
    getState: request<ZarfState>('cluster/state'),
    updateState: async (body: ZarfState) => request<ZarfState>('cluster/state', 'PUT', body),
}

const Packages = {

}

export { Cluster, Packages };

async function request<Response>(url: string, method: string = "GET", body?: any): Promise<Response> {
    try {
        const response = await fetch(`/${url}`, {
            method,
            headers: {
                'Authorization': `Bearer ${MAGIC}`,
                'Content-Type': 'application/json',
            },
            body,
        });
        const json = await response.json()
        return json as Response
    } catch (error) {
        console.error(error);
        return Promise.reject(error);
    }
}