import { test, expect, type Page } from "@playwright/test";

const ADMIN_EMAIL = "admin@demo.com";
const ADMIN_PASS = "123456";

// Login and wait for the main shell (sidebar) to appear.
// The app uses a hash router but doesn't change the hash on login —
// it re-renders via internal store state. So we wait for the sidebar
// instead of a URL change.
async function loginAndWaitForShell(page: Page) {
  await page.goto("/");
  // Wait for BootGate to finish loading (it calls auth.me, ui.meta, etc.)
  await expect(page.getByLabel("Email")).toBeVisible({ timeout: 15_000 });
  await page.getByLabel("Email").fill(ADMIN_EMAIL);
  await page.getByLabel("Password").fill(ADMIN_PASS);
  await page.getByRole("button", { name: /sign in/i }).click();
  // After login, BootGate renders <Shell> which contains the sidebar.
  await expect(page.locator("aside.sidebar")).toBeVisible({ timeout: 15_000 });
}

// TC-AUTH-01: Login with valid credentials
test("login form accepts valid credentials", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByLabel("Email")).toBeVisible({ timeout: 15_000 });
  await expect(page.getByLabel("Password")).toBeVisible();
  await page.getByLabel("Email").fill(ADMIN_EMAIL);
  await page.getByLabel("Password").fill(ADMIN_PASS);
  await page.getByRole("button", { name: /sign in/i }).click();
  // Shell (sidebar) appears after successful login
  await expect(page.locator("aside.sidebar")).toBeVisible({ timeout: 15_000 });
  // Login form should be gone
  await expect(page.getByLabel("Email")).not.toBeVisible();
});

// TC-DSH-01: Dashboard loads without error boundary
test("dashboard loads without error boundary", async ({ page }) => {
  await loginAndWaitForShell(page);
  // Dashboard is the default route — check main content area is visible
  await expect(page.locator("main.main")).toBeVisible();
  // Check no error boundary text
  await expect(page.locator("main.main")).not.toContainText("Something went wrong");
});

// TC-WEB-01: Websites list or empty state loads
test("websites view loads", async ({ page }) => {
  await loginAndWaitForShell(page);
  // Nav items are <button> elements inside the sidebar
  await page.locator("aside.sidebar").getByRole("button", { name: /websites/i }).click();
  // Wait for the page heading to appear
  await expect(page.locator("h1")).toContainText(/websites/i, { timeout: 10_000 });
  await expect(page.locator("main.main")).not.toContainText("Something went wrong");
});

// TC-PLG-01: Plugin registry shows gosite/mcp
test("plugin registry shows gosite/mcp", async ({ page }) => {
  await loginAndWaitForShell(page);
  await page.locator("aside.sidebar").getByRole("button", { name: /plugins/i }).click();
  await expect(page.locator("h1")).toContainText(/plugins/i, { timeout: 10_000 });
  await expect(page.locator("main.main")).toContainText(/gosite\/mcp/i);
});
