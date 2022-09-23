import { test, expect } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	page.on('pageerror', (err) => console.log(err.message));	
});

test.describe('view packages', () => {
	test('is initially blank', async ({ page }) => {
		await page.goto('/auth?token=insecure&next=/packages');
		await expect(page.locator('text=No deployed packages found 🙁')).toBeVisible();
		await expect(page.locator("a:has-text('Go Home')")).toHaveAttribute('href', '/');
	});
});
