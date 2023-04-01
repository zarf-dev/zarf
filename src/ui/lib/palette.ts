// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import type { Palettes } from '@ui';

export const ZarfPalettes: Palettes = {
	dark: {
		globalNav: '#0D133D',
		surface: '#0D133D',
		on: {
			globalNav: '#FFFFFF',
			surface: '#FFFFFF',
		},
		text: {
			primary: {
				onDark: '#FFFFFF',
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
		},
		navLinkSelectedBackground: 'rgba(74, 222, 222, 0.16)',
		chip: {
			color: 'var(--on-surface)',
			backgroundColor: 'var(--action-hover-on-dark)',
		},
	},
	light: {
		globalNav: '#0D133D',
		surface: '#0D133D',
		on: {
			globalNav: '#FFFFFF',
			surface: '#FFFFFF',
		},
		text: {
			primary: {
				onDark: '#FFFFFF',
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
		},
		navLinkSelectedBackground: 'rgba(74, 222, 222, 0.16)',
		chip: {
			color: 'var(--on-surface)',
			backgroundColor: 'var(--action-hover-on-dark)',
		},
	},
};
