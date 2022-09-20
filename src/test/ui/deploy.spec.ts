import { test, expect } from '@playwright/test';

test.describe('configure + review + deploy', () => {
	test.beforeEach(async ({ context }) => {
		// this is gross ⬇️
		await context.addInitScript(() => {
			window.sessionStorage.setItem('token', 'insecure');
		});
	});

	test('stepper renders', async ({ page }) => {
		await page.goto('/initialize/configure');

		const steps = await page.locator('.step > .step-icon');
		await expect(steps.nth(1)).toHaveClass(/disabled/);
		await expect(steps.nth(1)).toHaveText(/2/);
		await expect(steps.nth(2)).toHaveClass(/disabled/);
		await expect(steps.nth(2)).toHaveText(/3/);
	});

	test('component accordions render', async ({ page }) => {
		await page.goto('/initialize/configure');
		const components = await page.locator('.accordion');
		await expect(await components.count()).toBeGreaterThan(0);
	});

	// WIP: continue tomorrow
	// test('enable an optional package', async ({ page }) => {
	// 	await page.goto('/initialize/configure');
	// 	const checkbox = await page.locator("input[type='checkbox'][disabled='false']");
	// 	await checkbox.check();
	// 	await page.locator('text=review deployment').click();
	// 	await expect(await page.locator("input[type='checkbox']").nth(0).isChecked()).toBeTruthy();
	// });
});
