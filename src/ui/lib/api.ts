import type { ClusterSummary, ZarfState, ZarfPackage, ZarfDeployOptions } from './api-types';
import { HTTP } from './http';

const http = new HTTP();

const Cluster = {
	summary: () => http.get<ClusterSummary>('/cluster'),
	reachable: () => http.get<ZarfState>('/cluster/reachable'),
	hasZarf: () => http.get<ZarfState>('/cluster/has-zarf'),
	initialize: (body: ZarfDeployOptions) => http.put<boolean>('/cluster/initialize', body)
};

const State = {
	read: () => http.get<ZarfState>('/state'),
	update: (body: ZarfState) => http.patch<ZarfState>('/state', body)
};

const Packages = {
	find: () => http.get<string[]>('/packages/find'),
	findInHome: () => http.get<string[]>('/packages/find-in-home'),
	findInit: () => http.get<string[]>('/packages/find-init'),
	read: (name: string) => http.get<ZarfPackage>(`/packages/read/${encodeURIComponent(name)},`),
	readInit: () => http.get<ZarfPackage>('/packages/read/init'),
	getDeployedPackages: () => http.get<ZarfPackage[]>('/packages/list'),
	deploy: (body: ZarfDeployOptions) => http.put<boolean>('/packages/deploy', body),
	remove: (name: string) => http.del(`/packages/remove/${encodeURIComponent(name)}`)
};

export { Cluster, Packages, State };
