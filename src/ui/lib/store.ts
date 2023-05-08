// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { getPreferredTheme } from '@defense-unicorns/unicorn-ui';
import { writable } from 'svelte/store';
import { Cluster, Packages } from './api';
import type {
	APIDeployedPackageConnection,
	APIZarfPackage,
	ClusterSummary,
	DeployedPackage,
} from './api-types';

const pkgComponentDeployStore = writable<number[]>([]);
const pkgStore = writable<APIZarfPackage>();

// Theme Store

// check localStorage for theme, if not found, use the preferred theme, otherwise default to light
const storedTheme = localStorage.theme ?? getPreferredTheme(window) ?? 'light';

const themeStore = writable<'dark' | 'light'>(storedTheme);

// update localStorage when theme changes
themeStore.subscribe((theme) => {
	document.documentElement.setAttribute('data-theme', theme);
	localStorage.theme = theme;
});

// Cluster Summary Store
const clusterStore = writable<ClusterSummary | undefined>();

// Retrieves the cluster summary and stores it in the clusterStore
async function updateClusterSummary(): Promise<void> {
	try {
		const summary = await Cluster.summary();
		clusterStore.set(summary);
	} catch (err) {
		clusterStore.set(undefined);
	}
}

// Deployed Packages Store
const deployedPkgStore = writable<{ pkgs?: DeployedPackage[]; err?: Error } | undefined>();

// Stores DeployedPackages and an error if one occurs fetching them
async function updateDeployedPkgs(): Promise<void> {
	try {
		const pkgs = await Packages.getDeployedPackages();
		deployedPkgStore.set({ pkgs });
	} catch (err) {
		deployedPkgStore.set({ pkgs: [], err: err as Error });
	}
}

// Package Tunnels
interface PackageTunnels {
	[key: string]: APIDeployedPackageConnection[];
}
const tunnelStore = writable<PackageTunnels>({});

// Retrieves the list of package tunnels and stores them in the tunnelStore
async function updateConnections(): Promise<void> {
	try {
		const connections = await Packages.listConnections();
		tunnelStore.set(connections);
	} catch (err) {
		tunnelStore.set({});
	}
}

export {
	pkgComponentDeployStore,
	updateClusterSummary,
	type PackageTunnels,
	updateDeployedPkgs,
	deployedPkgStore,
	updateConnections,
	clusterStore,
	tunnelStore,
	themeStore,
	pkgStore,
};
