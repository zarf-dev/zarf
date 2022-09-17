import type { ClusterSummary, ZarfState, ZarfPackage, ZarfDeployOptions } from './api-types';
import { HTTP } from './http';

const http = new HTTP();

const Cluster = {
	summary: () => http.get<ClusterSummary>('/cluster'),
	reachable: () => http.get<ZarfState>('/cluster/reachable'),
	hasZarf: () => http.get<ZarfState>('/cluster/has-zarf'),
	getDeployedPackages: () => http.get<ZarfPackage[]>('/package/list'),
  initialize: (body: ZarfDeployOptions) => http.put<boolean>('/package/initialize', body)
};

const State = {
	read: () => http.get<ZarfState>('/state'),
	update: (body: ZarfState) => http.patch<ZarfState>('/state', body)
};

const Packages = {
	find: () => http.get<string[]>('/packages/find'),
	findInHome: () => http.get<string[]>('/packages/find-in-home'),
	read: (name: string) => http.get<string>(`/packages/read/${encodeURIComponent(name)}`)
};

export { Cluster, Packages, State };
