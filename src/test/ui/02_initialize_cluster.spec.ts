// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	page.on('pageerror', (err) => console.log(err.message));
});

const getToSelectPage = async (page) => {
	await page.goto('/auth?token=insecure&next=/packages?init=true', { waitUntil: 'networkidle' });
};

const getToConfigurePage = async (page) => {
	await getToSelectPage(page);
	// Find first init package deploy button.
	const deployInit = page.getByTitle('init').first();
	// click the init package deploy button.
	await deployInit.click();
};

const validateHorizontalStepperItems = async (page, activeIndex, steps) => {
	const stepperItems = await page.locator('.stepper .stepper-item .step');
	for (let i = 0; i < stepperItems.length; i++) {
		await expect(stepperItems.nth(i)).toContainText(steps[i]);
		if (activeIndex <= i) {
			await expect(stepperItems.nth(i).locator('.step-icon')).toHaveClass(/primary/);
		} else {
			await expect(stepperItems.nth(i).locator('.step-icon')).toHaveClass(/disabled/);
		}
	}
};

test.describe('initialize a zarf cluster', () => {
	test('Select, configure, and review init package @pre-init', async ({ page }) => {
		await getToSelectPage(page);

		await validateHorizontalStepperItems(page, 0, [
			'1 Select',
			'2 Configure',
			'3 Review',
			'4 Deploy',
		]);

		// Find first init package deploy button.
		const deployInit = page.getByTitle('init').first();
		// click the init package deploy button.
		await deployInit.click();
		// Components (check most functionaliy with k3s component)
		const k3s = page.locator('.accordion:has-text("k3s")');
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
			.locator('.accordion-header:has-text("git-server")')
			.locator('.deploy-component-toggle');
		await gitServerDeployToggle.click();
		await expect(gitServerDeployToggle).toHaveAttribute('aria-pressed', 'true');

		// Validate that components are maintained in the review package page.
		await page.locator('text=review deployment').click();

		await validateHorizontalStepperItems(page, 1, ['Select', 'Configure', '3 Review', '4 Deploy']);

		await validateRequiredCheckboxes(page);
	});

	test('deploy the init package @init', async ({ page }) => {
		await getToConfigurePage(page);
		await page.getByRole('link', { name: 'review deployment' }).click();
		await page.waitForURL('/packages/init/review');
		await page.getByRole('link', { name: 'deploy package' }).click();
		await page.waitForURL('/packages/init/deploy', { waitUntil: 'networkidle' });
		await validateHorizontalStepperItems(page, 2, ['Select', 'Configure', 'Review', '3 Deploy']);

		// expect all steps to have success class
		const stepperItems = page.locator('.stepper-vertical .step-icon');

		// deploy zarf-injector
		await expect(stepperItems.nth(0)).toHaveClass(/success/, {
			timeout: 45000,
		});
		// deploy zarf-seed-registry
		await expect(stepperItems.nth(1)).toHaveClass(/success/, {
			timeout: 45000,
		});
		// deploy zarf-registry
		await expect(stepperItems.nth(2)).toHaveClass(/success/, {
			timeout: 45000,
		});
		// deploy zarf-agent
		await expect(stepperItems.nth(3)).toHaveClass(/success/, {
			timeout: 45000,
		});

		// verify the final step succeeded
		await expect(page.locator('text=Deployment Succeeded')).toBeVisible({ timeout: 120000 });

		// then verify the page redirects to the packages dashboard
		await page.waitForURL('/', { timeout: 10000 });
	});
});

async function validateRequiredCheckboxes(page) {
	// Check remaining components for deploy states
	const injector = page.locator('.accordion-header:has-text("zarf-injector")');
	await expect(injector.locator('.deploy-component-toggle')).toBeHidden();

	const seedRegistry = page.locator('.accordion-header:has-text("zarf-seed-registry")');
	await expect(seedRegistry.locator('.deploy-component-toggle')).toBeHidden();

	const registry = page.locator('.accordion-header:has-text("zarf-registry")');
	await expect(registry.locator('.deploy-component-toggle')).toBeHidden();
}
