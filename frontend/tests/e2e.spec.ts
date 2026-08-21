import { test, expect } from "@playwright/test";
test("app shell loads", async ({ page }) => {
  await page.goto("http://localhost:1420");
  await expect(page.getByTestId("app-shell")).toBeVisible();
});
test("header bar and input hud visible", async ({ page }) => {
  await page.goto("http://localhost:1420");
  await expect(page.getByTestId("header-bar")).toBeVisible();
  await expect(page.getByTestId("input-hud")).toBeVisible();
});
test("run command button and reasoning panel", async ({ page }) => {
  await page.goto("http://localhost:1420");
  await expect(page.getByTestId("run-command")).toBeVisible();
  await expect(page.getByTestId("reasoning-panel")).toBeVisible();
});
test("keyboard shortcut Cmd+K focuses palette", async ({ page }) => {
  await page.goto("http://localhost:1420");
  await page.keyboard.press("Control+K");
  await expect(page.getByLabel("prompt")).toBeFocused();
});
