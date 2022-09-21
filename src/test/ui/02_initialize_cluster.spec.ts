import { test, expect } from '@playwright/test';

const checkbox = 'input[type=checkbox]';

test.describe('initialize a zarf cluster', () => {
	// this below store the current page in a higher scope, so each indivdual test below will use same page context
	let page;
	test.beforeAll(async ({ browser }) => {
		const context = await browser.newContext();
		page = await context.newPage();
		await page.goto('/auth?token=insecure');
	});

	test('configure the init package', async () => {
		await page.goto('/initialize/configure');
		await expect(page).toHaveTitle('Configure');

		// Stepper
		await expect(page.locator('.stepper :text("1 Configure") .step-icon')).toHaveClass(/primary/);
		await expect(page.locator('.stepper :text("2 Review") .step-icon')).toHaveClass(/disabled/);
		await expect(page.locator('.stepper :text("3 Deploy") .step-icon')).toHaveClass(/disabled/);

		// Package details
		await expect(page.locator('text=Package Type ZarfInitConfig')).toBeVisible();
		await expect(
			page.locator('text=Meta data Name: init Description: Used to establish a new Zarf cluster')
		).toBeVisible();

		// Components (check most functionaliy with k3s component)
		let k3s = page.locator('.accordion:has-text("k3s (Optional)")');
		await expect(k3s.locator(checkbox)).toBeEnabled();
		await expect(
			page.locator('.component-accordion-header:has-text("*** REQUIRES ROOT *** Install K3s")')
		).toBeVisible();
		await expect(k3s.locator('code')).toBeHidden();
		await k3s.locator('button').click();
		await expect(k3s.locator('code')).toBeVisible();
		await expect(k3s.locator('code:has-text("name: k3s")')).toBeVisible();

		// Check remaining components for deploy states
		let injector = page.locator('.accordion:has-text("zarf-injector (Required)")');
		await expect(injector.locator(checkbox)).toBeDisabled();
		await expect(injector.locator(checkbox)).toBeChecked();

		let seedRegistry = page.locator('.accordion:has-text("zarf-seed-registry (Required)")');
		await expect(seedRegistry.locator(checkbox)).toBeDisabled();
		await expect(seedRegistry.locator(checkbox)).toBeChecked();

		let registry = page.locator('.accordion:has-text("zarf-registry (Required)")');
		await expect(registry.locator(checkbox)).toBeDisabled();
		await expect(registry.locator(checkbox)).toBeChecked();

		let logging = page.locator('.accordion:has-text("logging (Optional)")');
		await expect(logging.locator(checkbox)).toBeEnabled();

		let gitServer = page.locator('.accordion:has-text("git-server (Optional)")');
		await expect(gitServer.locator(checkbox)).toBeEnabled();

		await page.locator('text=review deployment').click();
		await expect(page).toHaveURL('/initialize/review');
	});

	test('review the init package', async () => {
		await page.goto('/initialize/review');
		await expect(page).toHaveTitle('Review');

		// finish verifying the components are read-only and only include the required ones since we didn't select any optional ones
		const componentAccordions = await page.locator('.accordion');
		const componentAccordionsLen = await componentAccordions.count();
		await expect(componentAccordionsLen).toBe(4);
		for (let i = 0; i < componentAccordionsLen; i++) {
			const accordion = await componentAccordions.nth(i);
			await expect(await accordion.locator('.component-accordion-header')).toContainText(
				'(Required)'
			);
			await expect(accordion.locator(checkbox)).toBeDisabled();
		}
	});

	test('perform init package deployment', async () => {
		// todo: finish deploy test
	});
});
