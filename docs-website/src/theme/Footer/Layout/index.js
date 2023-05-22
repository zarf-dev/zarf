import React from 'react';

// Reorder and remove default styling from copyright, logo, and links.
export default function FooterLayout({ links, logo, copyright }) {
	return (
		<footer className="footer footer--dark">
			<div className="container container-fluid">
				{logo}
				{copyright}
				{links}
			</div>
		</footer>
	);
}
