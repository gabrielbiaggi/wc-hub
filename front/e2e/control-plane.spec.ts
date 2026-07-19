import { expect, test, type Page } from '@playwright/test'

const hourlyPassword = () => {
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'America/Sao_Paulo',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(new Date())
  const value = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? ''
  return `Hub${value('day')}${value('month')}${value('year')}${value('hour')}`
}

const login = async (page: Page) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Entrar no control plane' })).toBeVisible()
  await page.getByLabel('Email ou usuário').fill('allmight')
  await page.getByLabel('Senha').fill(hourlyPassword())
  await page.getByRole('button', { name: 'Autenticar' }).click()
  await expect(page.getByRole('heading', { name: 'Operations overview' })).toBeVisible()
}

test('master login and real integration routes render without server errors', async ({ page }) => {
  const failures: string[] = []
  const consoleErrors: string[] = []
  await login(page)
  page.on('response', (response) => {
    if (response.url().includes('/api/') && response.status() >= 500) failures.push(`${response.status()} ${response.url()}`)
  })
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })

  const routes: Array<[string, string]> = [
    ['/docker', 'Docker runtime'],
    ['/kubernetes', 'Kubernetes fabric'],
    ['/cloudflare', 'Cloudflare edge fabric'],
    ['/github', 'GitHub delivery'],
    ['/terraform', 'Terraform plans'],
    ['/proxmox', 'Proxmox fabric'],
    ['/storage', 'MergerFS browser'],
    ['/cloud', 'Oracle Cloud Infrastructure'],
  ]
  for (const [path, heading] of routes) {
    await page.goto(path)
    await expect(page.getByRole('heading', { name: heading })).toBeVisible()
  }
  expect(failures).toEqual([])
  expect(consoleErrors).toEqual([])
})
