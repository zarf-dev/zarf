import type { Palettes } from '@ui';

// Current default from @ui
export const ZarfPalettes: Palettes = {
	shared: {
		primary: '#68c4ff',
		secondary: '#787ff6',
		surface: '#244a8f',
		success: '#2e7d32',
		warning: '#ed6c02',
		info: '#0288d1',
		error: '#b00020',
		on: {
			primary: 'black',
			secondary: 'white',
			surface: '#ffffff',
			success: 'white',
			warning: 'white',
			info: 'white',
			error: 'white'
		},
		text: {
			primary: {
				onDark: 'white',
				onLight: 'rgb(0,0,0, 0.87)',
				onBackground: 'rgb(255, 255, 255, 0.87)'
			},
			secondary: {
				onLight: 'rgb(0,0,0, 0.54)',
				onDark: 'rgba(255, 255, 255, 0.7)',
				onBackground: 'rgba(255, 255, 255, 0.7)'
			},
			hint: {
				onLight: 'rgba(0, 0, 0, 0.38)',
				onDark: 'rgba(255, 255, 255, 0.5)',
				onBackground: 'rgba(255, 255, 255, 0.5)'
			},
			disabled: {
				onLight: 'rgba(0, 0, 0, 0.38)',
				onDark: 'rgba(255, 255, 255, 0.5)',
				onBackground: 'rgba(255, 255, 255, 0.5)'
			},
			icon: {
				onLight: 'rgba(0, 0, 0, 0.38)',
				onDark: 'rgba(255, 255, 255, 0.5)',
				onBackground: 'rgba(255, 255, 255, 0.5)'
			}
		}
	},
	dark: {
		background: '#0a0e2e',
		onBackground: '#ffffff'
	},
	light: {
		background: '#ffffff',
		surface: '#ffffff',
		on: {
			background: 'black',
			surface: 'black'
		}
	}
};
