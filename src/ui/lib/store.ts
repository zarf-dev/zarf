// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { writable } from 'svelte/store';
import type { APIZarfPackage, ClusterSummary } from './api-types';

const clusterStore = writable<ClusterSummary>();
const pkgStore = writable<APIZarfPackage>();
const pkgComponentDeployStore = writable<number[]>([]);
const themeStore = writable<'dark' | 'light'>('dark');

export { clusterStore, pkgStore, pkgComponentDeployStore, themeStore };
