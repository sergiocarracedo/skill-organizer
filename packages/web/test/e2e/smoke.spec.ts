import { expect, test } from "@playwright/test";

test("homepage and docs smoke flow", async ({ page }) => {
  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: /keep your skills organized/i }),
  ).toBeVisible();
  await expect(
    page
      .locator(".home-hero__actions")
      .getByRole("link", { name: /install guide/i }),
  ).toBeVisible();
  await expect(
    page.getByText("personal--coding--frontend-auditor"),
  ).toBeVisible();

  await page.getByRole("link", { name: /start with onboard/i }).click();

  await expect(page).toHaveURL(/\/docs\/reference\/onboard\/?$/);
  await expect(page.getByRole("heading", { name: "onboard" })).toBeVisible();
  await expect(page.getByText("skill-organizer onboard")).toBeVisible();

  await page.goto("/docs");

  await expect(page).toHaveURL(/\/docs$/);
  await expect(
    page.getByRole("heading", { name: /guides for the workflow/i }),
  ).toBeVisible();

  await page.locator('a[href="/docs/reference/onboard"]').click();
  await expect(page).toHaveURL(/\/docs\/reference\/onboard\/?$/);
  await expect(page.getByRole("heading", { name: "onboard" })).toBeVisible();
  await expect(page.getByText("skill-organizer onboard")).toBeVisible();
});
