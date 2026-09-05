import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  const baseURL = 'http://127.0.0.1:8080';

  test('should show login page and allow admin login', async ({ page }) => {
    await page.goto(`${baseURL}/`);
    await expect(page.locator('text=Login').first()).toBeVisible();

    const usernameInput = page.locator('input[type="text"]').first();
    const passwordInput = page.locator('input[type="password"]').first();
    
    await usernameInput.fill('admin');
    await passwordInput.fill('admin'); // Seed password

    await page.locator('button[type="submit"], button:has-text("Login")').first().click();

    await expect(page.locator('text=DocuNest Login').first()).toBeHidden({ timeout: 10000 });
  });

  test('should reject invalid credentials', async ({ page }) => {
    await page.goto(`${baseURL}/`);
    
    const usernameInput = page.locator('input[type="text"]').first();
    const passwordInput = page.locator('input[type="password"]').first();
    
    await usernameInput.fill('admin');
    await passwordInput.fill('wrongpassword');

    await page.locator('button[type="submit"], button:has-text("Login")').first().click();

    // Check for an error message or that we are still on the login page
    await expect(page.locator('text=Login').first()).toBeVisible();
  });
});
