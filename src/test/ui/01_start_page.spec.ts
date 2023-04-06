import { test, expect } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	page.on('pageerror', (err) => console.log(err.message));
});

test.describe('Landing Page', () => {
	test('Connect cluster @pre-init', async ({ page }) => {
		await page.goto('/auth?token=insecure', { waitUntil: 'networkidle' });

		// Expect cluster table to display not connected state
		const clusterInfo = page.locator('.cluster-not-connected');
		expect(await clusterInfo.textContent()).toContain('Cluster not connected');

		// Expect navdrawer cluster state to display not connected
		const navDrawerHeader = page.locator('.nav-drawer-header');
		expect(await navDrawerHeader.textContent()).toContain('Cluster not connected');

		// Expect the Packages Table to contain no packages
		const packageTableBody = page.locator('.package-list-body');
		expect(await packageTableBody.textContent()).toContain('No Packages have been Deployed');

		// Open Connect Cluster Dialog
		const connectClusterButton = page.locator('button:has-text("Connect Cluster")');
		await connectClusterButton.click();

		// Ensure Kubeconfig is found
		const kubeconfigDialog = page.locator('.dialog-content');
		expect(await kubeconfigDialog.textContent()).toContain('Kubeconfig Found');

		// Click Connect Cluster Anchor in the dialog to goto /packages?init=true
		const connectAnchor = kubeconfigDialog.locator('a:has-text("Connect Cluster")');
		await connectAnchor.click();

		await page.waitForURL('/packages?init=true');
	});

	// test('page redirects to /packages @post-init', async ({ page }) => {
	// 	await page.goto('/auth?token=insecure');

	// 	// display loading spinner
	// 	const spinner = page.locator('.spinner');
	// 	await expect(spinner).toBeVisible();

	// 	// spinner disappears
	// 	await expect(spinner).not.toBeVisible();

	// 	// expect to be redirected to /packages
	// 	await page.waitForURL('/packages', { timeout: 10000 });
	// });
});
