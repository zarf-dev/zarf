// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

import { expect, test } from '@playwright/test';


test.describe.serial('connect the dos-games package @connect', async () => {
    test.beforeEach(async ({ page }) => {
        page.on('pageerror', (err) => console.log(err.message));
        await page.goto('/auth?token=insecure', { waitUntil: 'networkidle' });
    });

    test('connect the dos-games package', async ({ page }) => {
        let menu = await openDosGamesMenu(page);

        // Ensure the menu contains the Connect option
        expect(await menu.textContent()).toContain('Connect...');

        const connect = menu.locator('span:text-is("Connect...")').first();

        // Open Connect Deployed Package Dialog
        await connect.click();

        const connectDialog = page.locator('.dialog-open');
        expect(await connectDialog.textContent()).toContain('Connect to Resource');
        const connectButton = connectDialog.locator('button:has-text("Connect")');

        // Click the Connect Button
        await connectButton.click();
        await page.waitForResponse('api/tunnels/connect/doom');

        menu = await openDosGamesMenu(page);
        expect(await menu.textContent()).toContain('Disconnect...');
    });

    test('disconnect the dos-games package', async ({page}) => {
        // Dispose context once it's no longer needed.
        let menu = await openDosGamesMenu(page);

        // Ensure the menu contains the Disconnect option
        expect(await menu.textContent()).toContain('Disconnect...');

        const disconnect = menu.locator('span:text-is("Disconnect...")');

        // Open Disconnect Deployed Package Dialog
        await disconnect.click();

        const dialog = page.locator('.dialog-open');
        expect(await dialog.textContent()).toContain('Disconnect Resource');
        const disconnectButton = dialog.locator('.button-label:text-is("Disconnect")');
        
        // Click the Disconnect Button
        await disconnectButton.click();

        // Ensure the menu no longer contains the Disconnect option
        menu = await openDosGamesMenu(page);
        expect(await menu.textContent()).not.toContain('Disconnect...');

    });

});

async function openDosGamesMenu(page: any) {
    // Find Dos Games Package in Packages Table
    const packageTableBody = page.locator('.package-list-body');
    const packageRow = packageTableBody.locator('.package-table-row:has-text("dos-games")');

    // Open the menu for the package
    const more = packageRow.locator('.more > button').first();
    await more.click();

    // Find the menu and return it
    const menu = page.locator('.menu.open');
    return menu;
}
