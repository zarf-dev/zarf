import { test, expect } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	await page.goto('/auth?token=insecure');
});

test.describe('view packages', () => {
	test('is initially blank', async ({ page }) => {
		await page.goto('/auth?token=insecure');
		await page.goto('/packages');
		await expect(page.locator('text=No deployed packages found 🙁')).toBeVisible();
		await expect(page.locator("a:has-text('Go Home')")).toHaveAttribute('href', '/');
	});
});
