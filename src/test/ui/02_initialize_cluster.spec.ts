import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	page.on('pageerror', (err) => console.log(err.message));
});

test.describe('initialize a zarf cluster', () => {
	test('configure the init package', async ({ page }) => {
		await page.goto('/auth?token=insecure&next=/initialize/configure');

		// Stepper
		await expect(page.locator('.stepper :text("1 Configure") .step-icon')).toHaveClass(/primary/);
		await expect(page.locator('.stepper :text("2 Review") .step-icon')).toHaveClass(/disabled/);
		await expect(page.locator('.stepper :text("3 Deploy") .step-icon')).toHaveClass(/disabled/);

		// Package details
		await expect(page.locator('.chip-wrapper:has-text("ZarfInitConfig")')).toBeVisible();
		await expect(
			page.locator('.mdc-typography--body2:has-text("Used to establish a new Zarf cluster")')
		).toBeVisible();

		// Components (check most functionaliy with k3s component)
		const k3s = page.locator('.accordion:has-text("k3s")');
		await expect(k3s.locator('.chip-wrapper:has-text("Optional")')).toBeVisible();
		await expect(k3s.locator('.deploy-component-toggle')).toHaveAttribute('aria-pressed', 'false');
		await k3s.locator('text=Deploy').click();
		await expect(k3s.locator('.deploy-component-toggle')).toHaveAttribute('aria-pressed', 'true');
		await expect(
			page.locator('.component-accordion-header:has-text("*** REQUIRES ROOT *** Install K3s")')
		).toBeVisible();
		await expect(k3s.locator('code')).toBeHidden();
		await k3s.locator('.accordion-toggle').click();
		await expect(k3s.locator('code')).toBeVisible();
		await expect(k3s.locator('code:has-text("name: k3s")')).toBeVisible();

		// Check remaining components for deploy states
		await validateRequiredCheckboxes(page);

		const loggingDeployToggle = page
			.locator('.accordion:has-text("logging")')
			.locator('.deploy-component-toggle');
		await loggingDeployToggle.click();
		await expect(loggingDeployToggle).toHaveAttribute('aria-pressed', 'true');

		const gitServerDeployToggle = page
			.locator('.accordion:has-text("git-server")')
			.locator('.deploy-component-toggle');
		await gitServerDeployToggle.click();
		await expect(gitServerDeployToggle).toHaveAttribute('aria-pressed', 'true');

		await page.locator('text=review deployment').click();
		await expect(page).toHaveURL('/initialize/review');
	});

	test('review the init package', async ({ page }) => {
		await page.goto('/auth?token=insecure&next=/initialize/review');

		await validateRequiredCheckboxes(page);
	});
});

async function validateRequiredCheckboxes(page) {
	// Check remaining components for deploy states
	const injector = page.locator('.accordion:has-text("zarf-injector")');
	expect(injector.locator('text=Deploy')).toBeHidden();

	const seedRegistry = page.locator('.accordion:has-text("zarf-seed-registry")');
	expect(seedRegistry.locator('text=Deploy')).toBeHidden();

	const registry = page.locator('.accordion:has-text("zarf-registry")');
	expect(registry.locator('text=Deploy')).toBeHidden();
}
