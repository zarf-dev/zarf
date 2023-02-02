// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import type { ThemeTypography } from '@ui';
import { UUI_TYPOGRAPHY } from '@ui';

// custom typography from figma only for Zarf UI
const extra = {
	body3: {
		fontWeight: '400',
		fontSize: '14px',
		lineHeight: '143%',
		fontStyle: 'normal',
		letterSpacing: '.17px',
	},
	th: {
		fontStyle: 'normal',
		fontWeight: '500',
		fontSize: '0.875em',
		lineHeight: '24px',
		letterSpacing: '0.17px',
	}
};

export const ZarfTypography: ThemeTypography = {
	...UUI_TYPOGRAPHY,
	...extra,
	// custom typography from figma
	body1: {
		fontSize: '16px',
		fontWeight: '400',
		lineHeight: '120%',
		letterSpacing: '0.15px'
	},
	body2: {
		fontSize: '14px',
		fontWeight: '400',
		lineHeight: '120%',
		letterSpacing: '0.17px'
	},
	subtitle1: {
		fontSize: '16px',
		fontWeight: '500',
		lineHeight: '150%',
		letterSpacing: '0.15px'
	},
	subtitle2: {
		fontSize: '14px',
		fontWeight: '500',
		lineHeight: '140%',
		letterSpacing: '0.1px'
	},
	caption: {
		fontSize: '12px',
		fontWeight: '400',
		lineHeight: '166%',
		letterSpacing: '0.4px'
	},
	code: {
		fontSize: '14px',
		fontWeight: '400',
		lineHeight: '143%',
		letterSpacing: '0.17px',
		fontFamily: 'monospace, monospace'
	},
	overline: {
		fontSize: '12px',
		fontWeight: '400',
		lineHeight: '266%',
		letterSpacing: '1px',
		textTransform: 'uppercase'
	},
	h1: {
		fontSize: '96px',
		fontWeight: '300',
		lineHeight: '116.7%',
		letterSpacing: '-1.5px'
	},
	h2: {
		fontSize: '60px',
		fontWeight: '300',
		lineHeight: '120%',
		letterSpacing: '-0.5px'
	},
	h3: {
		fontSize: '48px',
		fontWeight: '400',
		lineHeight: '116.7%',
		letterSpacing: '-0.25px'
	},
	h4: {
		fontSize: '34px',
		fontWeight: '400',
		lineHeight: '123.5%',
		letterSpacing: '0.25px'
	},
	h5: {
		fontSize: '24px',
		fontWeight: '500',
		lineHeight: '133.4%'
	},
	h6: {
		fontSize: '20px',
		fontWeight: '400',
		lineHeight: '160%',
		letterSpacing: '0.15px'
	}
};
