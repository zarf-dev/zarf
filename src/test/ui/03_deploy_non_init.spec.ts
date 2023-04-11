import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	page.on('pageerror', (err) => console.log(err.message));
});

const getToSelectPage = async (page) => {
	await page.goto('/auth?token=insecure&next=/packages', { waitUntil: 'networkidle' });
};

const getToReview = async (page) => {
	await getToSelectPage(page);
	// Find first dos-games package deploy button.
	const dosGames = page.getByTitle('dos-games').first();
	// click the dos-games package deploy button.
	await dosGames.click();
	await page.getByRole('link', { name: 'review deployment' }).click();
	await page.waitForURL('/packages/dos-games/review');
};

test('deploy the dos-games package @post-init', async ({ page }) => {
	await getToReview(page);
	await page.getByRole('link', { name: 'deploy' }).click();
	await page.waitForURL('/packages/dos-games/deploy', { waitUntil: 'networkidle' });

	// verify the deployment succeeded
	await expect(page.locator('text=Deployment Succeeded')).toBeVisible({ timeout: 120000 });

	// then verify the page redirects to the Landing Page
	await page.waitForURL('/', { timeout: 10000 });
});
