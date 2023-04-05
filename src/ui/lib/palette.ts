// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import type { Palettes } from '@ui';

export const ZarfPalettes: Palettes = {
	dark: {
		primary: '#4ADEDE',
		globalNav: '#0D133D',
		surface: '#0D133D',
		on: {
			globalNav: '#FFFFFF',
			surface: '#FFFFFF',
		},
		text: {
			primary: {
				onDark: '#FFFFFF',
				onLight: 'rgba(0, 0, 0, 0.87)',
			},
			secondary: {
				onDark: 'rgba(255, 255, 255, 0.7)',
			},
		},
		action: {
			hover: {
				onDark: 'rgba(255, 255, 255, .08)',
			},
			selected: {
				onDark: 'rgba(255, 255, 255, 0.16)',
			},
			active: {
				'56p': 'rgba(255, 255, 255, 0.56)',
			},
		},
		chip: {
			color: 'var(--on-surface)',
			backgroundColor: 'var(--action-hover-on-dark)',
		},
		shades: {
			primary: {
				'16p': 'rgba(74, 222, 222, 0.16)',
			},
		},
		grey: {
			300: 'rgba(224, 224, 224, 1)',
		},
		blue: {
			200: 'rgba(144, 202, 249, 1)',
		},
	},
	light: {
		primary: '#4ADEDE',
		globalNav: '#0D133D',
		surface: '#0D133D',
		on: {
			globalNav: '#FFFFFF',
			surface: '#FFFFFF',
		},
		text: {
			primary: {
				onDark: '#FFFFFF',
				onLight: 'rgba(0, 0, 0, 0.87)',
			},
			secondary: {
				onDark: 'rgba(255, 255, 255, 0.7)',
			},
		},
		action: {
			hover: {
				onDark: 'rgba(255, 255, 255, .08)',
			},
			selected: {
				onDark: 'rgba(255, 255, 255, 0.16)',
			},
			active: {
				'56p': 'rgba(255, 255, 255, 0.56)',
			},
		},
		chip: {
			color: 'var(--on-surface)',
			backgroundColor: 'var(--action-hover-on-dark)',
		},
		shades: {
			primary: {
				'16p': 'rgba(74, 222, 222, 0.16)',
			},
		},
		grey: {
			300: 'rgba(224, 224, 224, 1)',
		},
		blue: {
			200: 'rgba(144, 202, 249, 1)',
		},
	},
};
