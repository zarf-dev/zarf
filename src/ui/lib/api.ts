// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import type {
	APIPackageConnections,
	APIZarfDeployPayload,
	APIZarfPackage,
	ClusterSummary,
	DeployedComponent,
	DeployedPackage,
	ZarfState,
} from './api-types';
import { HTTP, type EventParams } from './http';

const http = new HTTP();

const Auth = {
	connect: async (token: string) => {
		if (!token) {
			return false;
		}

		http.updateToken(token);
		return await http.head('/');
	},
};

const Cluster = {
	summary: () => http.get<ClusterSummary>('/cluster'),
	state: {
		read: () => http.get<ZarfState>('/state'),
		update: (body: ZarfState) => http.patch<ZarfState>('/state', body),
	},
};

const Packages = {
	find: () => http.get<string[]>('/packages/find'),
	findInHome: () => http.get<string[]>('/packages/find-in-home'),
	findInit: () => http.get<string[]>('/packages/find-init'),
	read: (name: string) => http.get<APIZarfPackage>(`/packages/read/${encodeURIComponent(name)}`),
	getDeployedPackages: () => http.get<DeployedPackage[]>('/packages/list'),
	packageConnections: (name: string) => http.get<APIPackageConnections>(`/packages/list/connections/${name}`),
	deploy: (options: APIZarfDeployPayload) => http.put<boolean>(`/packages/deploy`, options),
	deployStream: (eventParams: EventParams) =>
		http.eventStream('/packages/deploy-stream', eventParams),
	remove: (name: string) => http.del(`/packages/remove/${encodeURIComponent(name)}`),
};

const DeployingComponents = {
	list: () => http.get<DeployedComponent[]>('/components/deployed'),
};

export { Auth, Cluster, Packages, DeployingComponents };
