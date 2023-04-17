import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	page.on('pageerror', (err) => console.log(err.message));
});

const getToConfigurePage = async (page) => {
	await page.goto('/auth?token=insecure&next=/');
	await page.getByRole('link', { name: 'Initialize Cluster' }).click();
	await page.waitForURL('/package/init/configure');
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
	test('configure the init package @pre-init', async ({ page }) => {
		await getToConfigurePage(page);

		await validateHorizontalStepperItems(page, 0, ['1 Configure', '2 Review', '3 Deploy']);

		// Package details
		await expect(page.locator('text=Package Type ZarfInitConfig')).toBeVisible();
		await expect(
			page.locator('text=METADATA Name: Init Description: Used to establish a new Zarf cluster')
		).toBeVisible();

		// Components (check most functionaliy with k3s component)
		let k3s = page.locator('.accordion:has-text("k3s (Optional)")');
		await expect(k3s.locator('.deploy-component-toggle')).toHaveAttribute('aria-pressed', 'false');
		await k3s.locator('text=Deploy').click();
		await expect(k3s.locator('.deploy-component-toggle')).toHaveAttribute('aria-pressed', 'true');
		await expect(
			page.locator('.component-accordion-header:has-text("*** REQUIRES ROOT (not sudo) *** Install K3s")')
		).toBeVisible();
		await expect(k3s.locator('code')).toBeHidden();
		await k3s.locator('.accordion-toggle').click();
		await expect(k3s.locator('code')).toBeVisible();
		await expect(k3s.locator('code:has-text("name: k3s")')).toBeVisible();

		// Check remaining components for deploy states
		await validateRequiredCheckboxes(page);

		let loggingDeployToggle = page
			.locator('.accordion:has-text("logging (Optional)")')
			.locator('.deploy-component-toggle');
		await loggingDeployToggle.click();
		await expect(loggingDeployToggle).toHaveAttribute('aria-pressed', 'true');

		let gitServerDeployToggle = page
			.locator('.accordion:has-text("git-server (Optional)")')
			.locator('.deploy-component-toggle');
		await gitServerDeployToggle.click();
		await expect(gitServerDeployToggle).toHaveAttribute('aria-pressed', 'true');

		await page.locator('text=review deployment').click();
		await expect(page).toHaveURL('/package/init/review');
	});

	test('review the init package @pre-init', async ({ page }) => {
		await getToConfigurePage(page);

		await page.locator('text=review deployment').click();

		await validateHorizontalStepperItems(page, 1, ['Configure', '2 Review', '3 Deploy']);

		await validateRequiredCheckboxes(page);
	});

	test('deploy the init package @init', async ({ page }) => {
		await getToConfigurePage(page);
		await page.getByRole('link', { name: 'review deployment' }).click();
		await page.waitForURL('/package/init/review');
		await page.getByRole('link', { name: 'deploy' }).click();
		await page.waitForURL('/package/init/deploy');
		await validateHorizontalStepperItems(page, 2, ['Configure', 'Review', '3 Deploy']);

		// expect all steps to have success class
		const stepperItems = page.locator('.stepper-vertical .step-icon');

		// deploy zarf-injector
		await expect(stepperItems.nth(0)).toHaveClass(/success/, {
			timeout: 45000
		});
		// deploy zarf-seed-registry
		await expect(stepperItems.nth(1)).toHaveClass(/success/, {
			timeout: 45000
		});
		// deploy zarf-registry
		await expect(stepperItems.nth(2)).toHaveClass(/success/, {
			timeout: 45000
		});
		// deploy zarf-agent
		await expect(stepperItems.nth(3)).toHaveClass(/success/, {
			timeout: 45000
		});

		// verify the final step succeeded
		await expect(page.locator('text=Deployment Succeeded')).toBeVisible();

		// then verify the page redirects to the packages dashboard
		await page.waitForURL('/packages', { timeout: 10000 });
	});
});

async function validateRequiredCheckboxes(page) {
	// Check remaining components for deploy states
	const injector = page.locator('.accordion:has-text("zarf-injector (Required)")');
	await expect(injector.locator('.deploy-component-toggle')).toBeHidden();

	const seedRegistry = page.locator('.accordion:has-text("zarf-seed-registry (Required)")');
	await expect(seedRegistry.locator('.deploy-component-toggle')).toBeHidden();

	const registry = page.locator('.accordion:has-text("zarf-registry (Required)")');
	await expect(registry.locator('.deploy-component-toggle')).toBeHidden();
}
