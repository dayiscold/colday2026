import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';
import type { Release } from '../frontend/src/types';

const cover = (color: string) => 'data:image/svg+xml,' + encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="600" height="600"><rect width="600" height="600" fill="${color}"/><circle cx="300" cy="290" r="170" fill="#eee0ca"/><path d="M40 500 320 80 560 500Z" fill="#171b20"/><text x="40" y="560" fill="#eee0ca" font-family="monospace" font-size="38">COLDAY / AUDIO</text></svg>`);
const release = (id: number, overrides: Partial<Release> = {}): Release => ({
  id, title: `Релиз ${id}`, release_type: 'single', cover: cover('#452c3b'),
  bandlink: 'https://band.link/colday-test', upc: '', original_date: '2026-10-01',
  counter_songs: 1, song_duration: '00:03:20', featured: false, ...overrides,
});
const releases = [
  release(4, { title: 'Свет среди холода', featured: true, cover: cover('#526970') }),
  release(3, { title: 'Тишина', cover: cover('#86544a') }),
  release(2, { title: 'После полуночи', release_type: 'EP', cover: cover('#525166') }),
  release(1, { title: 'Холодный мир', cover: cover('#52674c') }),
];
async function mockReleases(page: Page, data: Release[] | null = releases) {
  await page.route('**/api/releases', route => route.fulfill({ json: data }));
}

test('renders releases and preserves destinations and featured selection', async ({ page }) => {
  // Deliberately put the featured release after another item to exercise selection.
  await mockReleases(page, [releases[1], releases[0], releases[2]]);
  await page.goto('/');
  await expect(page.locator('.featured-release h3')).toHaveText('Свет среди холода');
  await expect(page.locator('.catalog-release')).toHaveCount(2);
  await expect(page.locator('.hero-cta')).toContainText('Новинка — «Свет среди холода» single');
  await expect(page.locator('.hero-cta')).toHaveAttribute('href', '#music');
  await expect(page.locator('.featured-release .listen')).toHaveAttribute('href', releases[0].bandlink);
  await expect(page.getByRole('link', { name: 'COLDAY в Telegram' })).toHaveAttribute('href', 'https://t.me/coldaybigstepper');
  await expect(page.locator('.mail')).toHaveAttribute('href', 'mailto:deadplugbooking@gmail.com');
  await page.locator('.nav-sections a[href="#contacts"]').click();
  await expect(page).toHaveURL(/#contacts$/);
});

test('falls back to the first release and omits absent listening links', async ({ page }) => {
  await mockReleases(page, [release(1, { bandlink: '' }), release(2)]);
  await page.goto('/');
  await expect(page.locator('.featured-release h3')).toHaveText('Релиз 1');
  await expect(page.locator('.featured-release .listen')).toHaveCount(0);
});

test('announces loading, empty, and network error states', async ({ page }) => {
  let respond!: () => void;
  const pending = new Promise<void>(resolve => { respond = resolve; });
  await page.route('**/api/releases', async route => { await pending; await route.fulfill({ json: [] }); });
  await page.goto('/');
  await expect(page.getByRole('status')).toContainText('Загрузка релизов');
  respond();
  await expect(page.getByRole('status')).toContainText('Новые релизы скоро появятся здесь');
  await expect(page.locator('.hero-cta')).toContainText('Новые релизы');
  await page.unroute('**/api/releases');
  await page.route('**/api/releases', route => route.fulfill({ status: 500 }));
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.getByRole('status')).toContainText('Не удалось загрузить релизы');
});

test('accepts a null empty Go list and rejects malformed responses', async ({ page }) => {
  await mockReleases(page, null);
  await page.goto('/');
  await expect(page.getByRole('status')).toContainText('Новые релизы скоро');
  await page.unroute('**/api/releases');
  await page.route('**/api/releases', route => route.fulfill({ json: { error: 'invalid' } }));
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.getByRole('status')).toContainText('Не удалось загрузить');
});

test('refreshes on focus, retains data on failure, and resets the CTA when emptied', async ({ page }) => {
  let response = releases;
  let fail = false;
  await page.route('**/api/releases', route => fail ? route.fulfill({ status: 503 }) : route.fulfill({ json: response }));
  await page.goto('/');
  await expect(page.locator('.featured-release h3')).toHaveText(releases[0].title);
  fail = true;
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.getByRole('status')).toContainText('Не удалось');
  await expect(page.locator('.featured-release h3')).toHaveText(releases[0].title);
  fail = false;
  response = [release(5, { title: 'Обновлённый релиз' })];
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.locator('.featured-release h3')).toHaveText('Обновлённый релиз');
  await expect(page.getByRole('status')).toHaveCount(0);
  response = [];
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.locator('.hero-cta')).toHaveText('Новые релизы↙');
  await expect(page.locator('.release')).toHaveCount(0);
});

test('a delayed older request cannot replace a newer response', async ({ page }) => {
  await mockReleases(page);
  await page.goto('/');
  await expect(page.locator('.featured-release')).toBeVisible();
  await page.unroute('**/api/releases');
  let releaseOld!: () => void;
  let oldStarted!: () => void;
  const oldPending = new Promise<void>(resolve => { releaseOld = resolve; });
  const started = new Promise<void>(resolve => { oldStarted = resolve; });
  let count = 0;
  await page.route('**/api/releases', async route => {
    if (++count === 1) {
      oldStarted();
      await oldPending;
      await route.fulfill({ json: [release(1, { title: 'Старый ответ' })] }).catch(() => {});
    } else await route.fulfill({ json: [release(2, { title: 'Свежий ответ' })] });
  });
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await started;
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.locator('.featured-release h3')).toHaveText('Свежий ответ');
  releaseOld();
  await expect(page.locator('.featured-release h3')).toHaveText('Свежий ответ');
});

test('missing covers retain layout and recover when the image URL changes', async ({ page }) => {
  await page.route('**/broken-cover.png', route => route.fulfill({ status: 404 }));
  await mockReleases(page, [release(1, { cover: '/broken-cover.png' }), release(2, { cover: '' })]);
  await page.goto('/');
  await expect(page.getByRole('img', { name: 'Обложка отсутствует' })).toHaveCount(2);
  await page.unroute('**/api/releases');
  await mockReleases(page, [release(1)]);
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.locator('.featured-release .cover img')).toBeVisible();
});

test('motion preference persists, reduced motion disables effects, and keyboard focus works', async ({ page }) => {
  await mockReleases(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Приостановить эффекты' }).click();
  await expect(page.locator('.site')).toHaveAttribute('data-motion', 'paused');
  await page.reload();
  await expect(page.getByRole('button', { name: 'Включить эффекты' })).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: 'Включить эффекты' }).click();
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await expect(page.locator('.motion-toggle')).toBeHidden();
  await expect(page.locator('.site')).toHaveAttribute('data-motion', 'paused');
  await expect(page.locator('.cursor').first()).toHaveCSS('animation-name', 'none');
  await page.reload();
  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: 'Перейти к содержимому' })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.locator('main')).toBeFocused();
});

test('cover interference responds to hover and keyboard focus and honors pause', async ({ page }) => {
  await mockReleases(page);
  await page.goto('/');
  const cover = page.locator('.featured-release .cover');
  await cover.hover();
  await expect(cover.locator('.cover-noise')).toHaveCSS('animation-name', 'interference');
  await page.getByRole('button', { name: 'Приостановить эффекты' }).click();
  await cover.hover();
  await expect(cover.locator('.cover-noise')).toHaveCSS('animation-name', 'none');
  await page.getByRole('button', { name: 'Включить эффекты' }).click();
  await page.mouse.move(0, 0);
  await page.keyboard.press('Tab');
  await page.locator('.featured-release .listen').focus();
  await expect(cover.locator('.cover-noise')).toHaveCSS('animation-name', 'interference');
});

for (const [width, height] of [[320, 740], [390, 844], [768, 1024], [1440, 900], [2560, 1440]]) {
  test(`layout and screenshot ${width}x${height}`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height });
    await mockReleases(page);
    await page.goto('/');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await expect(page.locator('.release')).toHaveCount(4);
    await page.evaluate(() => document.fonts.ready);
    await expect(page.locator('.portrait img')).toHaveJSProperty('complete', true);
    for (const link of await page.locator('.nav-sections a').all()) await expect(link).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const title = (await page.locator('h1').boundingBox())!;
    const portrait = (await page.locator('.portrait').boundingBox())!;
    const cta = (await page.locator('.hero-cta').boundingBox())!;
    expect(Math.abs(cta.x + cta.width / 2 - width / 2)).toBeLessThan(2);
    expect(cta.y).toBeGreaterThan(title.y + title.height);
    if (width > 1024) expect(portrait.x).toBeGreaterThanOrEqual(title.x + title.width - 1);
    else expect(portrait.y + portrait.height).toBeLessThan(title.y);
    await page.screenshot({ path: testInfo.outputPath(`public-${width}.png`), fullPage: true });
    await page.unroute('**/api/releases');
    await mockReleases(page, [release(1, { title: 'ОченьДлинноеНазваниеБезПробелов'.repeat(8) }), release(2, { title: 'Длинное название '.repeat(15) })]);
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(page.locator('.release')).toHaveCount(2);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const overflow = await page.locator('main h3, .hero-cta').evaluateAll(elements => elements.some(element => element.scrollWidth > element.clientWidth + 1));
    expect(overflow).toBe(false);
  });
}

test('the production Go server serves the app, alias, assets, API and admin', async ({ page, request }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/static/colday.html');
  await expect(page.getByRole('heading', { name: 'COLDAY', exact: true })).toBeVisible();
  await expect(page.getByRole('status')).toContainText('Новые релизы скоро');
  expect(await page.locator('.portrait img').evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
  expect((await request.get('/admin')).status()).toBe(200);
  expect((await request.get('/admin/')).status()).toBe(200);
  expect((await request.get('/not-a-route')).status()).toBe(404);
  expect((await request.post('/api/releases', { data: { title: 'Unauthorized' } })).status()).toBe(401);
  expect(errors).toEqual([]);
});
