import { test, expect } from '@playwright/test';

test.describe('homepage', () => {
	test.beforeEach(async ({ context }) => {
		// this is gross ⬇️
		await context.addInitScript(() => {
			window.sessionStorage.setItem('token', 'insecure');
		});
	});

	test('has `Zarf UI` in title', async ({ page }) => {
		await page.goto('/');

		// Expect a title "to contain" a substring.
		await expect(page).toHaveTitle(/Zarf UI/);
	});

	test('spinner loads properly, then displays init btn', async ({ page }) => {
		await page.goto('/');

		const clusterSelector = page.locator('#cluster-selector');
		await expect(clusterSelector).toBeEmpty();

		const spinner = page.locator('.spinner');
		await expect(spinner).toBeVisible();

		const initBtn = page.locator('#init-cluster');
		await expect(initBtn).toHaveAttribute('href', '/initialize/configure');
		await expect(initBtn).toBeEnabled();

		const currentCluster = await clusterSelector.textContent();
		await expect(currentCluster).toBeTruthy();

		await expect(spinner).not.toBeVisible();
	});
});
