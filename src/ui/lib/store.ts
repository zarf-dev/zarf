import { writable } from 'svelte/store';
import type { APIZarfPackage, ClusterSummary } from './api-types';

const clusterStore = writable<ClusterSummary>();
const pkgStore = writable<APIZarfPackage>();
const pkgComponentDeployStore = writable<number[]>([]);

export { clusterStore, pkgStore, pkgComponentDeployStore };
