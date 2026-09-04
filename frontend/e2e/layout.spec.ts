import { expect, test } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'

const screenshotDir = resolve(process.cwd(), '../output/playwright/screenshots')

for (const viewport of [{ width: 1080, height: 700 }, { width: 900, height: 600 }]) {
  test(`desktop layout ${viewport.width}x${viewport.height}`, async ({ page }) => {
    await page.setViewportSize(viewport)
    await page.goto('/?scenario=many')
    await expect(page.getByRole('heading', { name: '固定端口' })).toBeVisible()
    await expect(page.getByTestId('summary-line')).toBeVisible()
    await expect(page.locator('tbody tr')).toHaveCount(24)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  })
}

test('covers empty, loading and error states', async ({ page }) => {
  await page.goto('/?scenario=empty')
  await expect(page.getByTestId('empty-state')).toBeVisible()
  await page.goto('/?scenario=loading')
  await expect(page.getByTestId('loading-state')).toBeVisible()
  await page.goto('/?scenario=error')
  await expect(page.getByText('节点未连接')).toBeVisible()
})

test('switches light and dark theme', async ({ page }) => {
  await page.goto('/')
  await page.getByTitle('切换深色主题').click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
})

test('renders settings content', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: '设置' }).click()
  await expect(page.getByRole('heading', { name: '设置' })).toBeVisible()
  await expect(page.getByText('登录系统后自动启动')).toBeVisible()
  await expect(page.getByText('客户端与数据')).toBeVisible()
  await expect(page.getByRole('button', { name: '保存设置' })).toBeVisible()
})

test('exports light and dark visual samples', async ({ page }) => {
  mkdirSync(screenshotDir, { recursive: true })
  await page.setViewportSize({ width: 1080, height: 700 })
  await page.goto('/?scenario=many')
  await expect(page.locator('tbody tr')).toHaveCount(24)
  await page.screenshot({ path: resolve(screenshotDir, 'nblink-light-1080x700.png'), fullPage: true })
  await page.setViewportSize({ width: 900, height: 600 })
  await page.screenshot({ path: resolve(screenshotDir, 'nblink-light-900x600.png'), fullPage: true })

  await page.getByTitle('切换深色主题').click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.locator('.toast')).toBeHidden({ timeout: 5_000 })
  await page.screenshot({ path: resolve(screenshotDir, 'nblink-dark-900x600.png'), fullPage: true })
  await page.setViewportSize({ width: 1080, height: 700 })
  await page.screenshot({ path: resolve(screenshotDir, 'nblink-dark-1080x700.png'), fullPage: true })
})
