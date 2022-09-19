import { writable } from 'svelte/store';
import type { ClusterSummary, ZarfPackage } from './api-types';

const clusterStore = writable<ClusterSummary>();
const pkgStore = writable<ZarfPackage>();
const pkgComponentDeployStore = writable<number[]>([]);
const pkgPath = writable<string[]>();

export { clusterStore, pkgStore, pkgComponentDeployStore, pkgPath };
