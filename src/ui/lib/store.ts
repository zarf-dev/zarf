import { writable } from 'svelte/store';
import type { ZarfPackage } from './api-types';

const pkgPath = writable<string[]>();
const pkgStore = writable<ZarfPackage>();
const pkgComponentDeployStore = writable<number[]>([]);

export { pkgStore, pkgComponentDeployStore, pkgPath };
