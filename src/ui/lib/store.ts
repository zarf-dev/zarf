// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { getPreferredTheme } from '@defense-unicorns/unicorn-ui';
import { writable } from 'svelte/store';
import type { APIZarfPackage, ClusterSummary, DeployedPackage } from './api-types';

const clusterStore = writable<ClusterSummary | undefined>();
const pkgStore = writable<APIZarfPackage>();
const deployedPkgStore = writable<{ pkgs?: DeployedPackage[]; err?: Error } | undefined>();
const pkgComponentDeployStore = writable<number[]>([]);
// check localstorage for theme, if not found, use the preferred theme, otherwise default to light
const storedTheme = localStorage.theme ?? getPreferredTheme(window) ?? 'light';
const themeStore = writable<'dark' | 'light'>(storedTheme);
// update localstorage when theme changes
themeStore.subscribe((theme) => {
	document.documentElement.setAttribute('data-theme', theme);
	localStorage.theme = theme;
});

export { clusterStore, pkgStore, pkgComponentDeployStore, themeStore, deployedPkgStore };
