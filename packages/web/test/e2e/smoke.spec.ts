import { expect, test } from '@playwright/test'

test('home and documentation navigation', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: /organize skills.*keep tools compatible/i })).toBeVisible()
  await page.getByRole('navigation', { name: 'Main navigation' }).getByRole('link', { name: 'Docs' }).click()
  await expect(page).toHaveURL(/\/docs\/?$/)
  await page.locator('a[href$="/docs/reference/onboard"]').first().click()
  await expect(page.getByRole('heading', { name: 'onboard', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Copy code' }).first()).toBeVisible()
})

test('theme preference persists across pages', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.goto('/')
  await page.getByRole('button', { name: 'Switch to light theme' }).click()
  await expect(page.locator('meta[name="color-scheme"]')).toHaveAttribute('content', 'light')
  await page.goto('/docs')
  await expect(page.locator('meta[name="color-scheme"]')).toHaveAttribute('content', 'light')
})

test('social metadata is present on home and documentation pages', async ({ page, request }) => {
  for (const path of ['/', '/docs/reference/onboard/']) {
    await page.goto(path)
    const title = await page.title()
    const description = await page.locator('meta[name="description"]').getAttribute('content')
    await expect(page.locator('meta[property="og:title"]')).toHaveAttribute('content', title)
    await expect(page.locator('meta[name="twitter:title"]')).toHaveAttribute('content', title)
    await expect(page.locator('meta[property="og:description"]')).toHaveAttribute('content', description!)
    await expect(page.locator('meta[property="og:site_name"]')).toHaveAttribute('content', 'Skill Organizer')
    await expect(page.locator('meta[name="twitter:card"]')).toHaveAttribute('content', 'summary_large_image')
    const image = await page.locator('meta[property="og:image"]').getAttribute('content')
    await expect(page.locator('meta[name="twitter:image"]')).toHaveAttribute('content', image!)
    expect(image).toBe('https://skill-organizer.sireno.dev/og/skill-organizer-social.png')
    const response = await request.get('/og/skill-organizer-social.png')
    expect(response.ok()).toBe(true)
    expect(response.headers()['content-type']).toContain('image/png')
  }
})

test('mobile navigation opens and closes', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  const toggle = page.getByRole('button', { name: /menu/i })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await expect(page.getByRole('navigation', { name: 'Main navigation' }).getByRole('link', { name: 'Docs' })).toBeVisible()
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
})

test('shared labels, links and documentation selection use the violet accent', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.goto('/')
  const accentColor = await page.getByText('Smart features', { exact: true }).evaluate((node) => getComputedStyle(node).color)
  for (const label of ['Works with', 'Managed skill updates', 'See skill commands', 'Stop managing skills in a flat folder']) {
    await expect(page.getByText(label, { exact: true }).first()).toHaveCSS('color', accentColor)
  }
  await expect(page.locator('.feature-card .accent-link').first()).toHaveCSS('color', accentColor)
  await page.goto('/docs/reference/onboard/')
  const current = page.locator('aside a[aria-current="page"]')
  await expect(current).toHaveCSS('color', accentColor)
})
