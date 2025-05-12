// E2E test: Uploads an MP3 and checks for transcription using Playwright

import { expect, test } from '@playwright/test';
import { startAppContainer, stopAppContainer } from './containerSetup';

let appUrl: string;

test.beforeAll(async () => {
  const { host, port } = await startAppContainer();
  appUrl = `http://${host}:${port}`;
});

test.afterAll(async () => {
  await stopAppContainer();
});

test('upload MP3 and get transcription', async ({ page }) => {
  // Visit the app (containerized)
  await page.goto(appUrl);

  // Wait for transcription result (adjust selector as needed)
  const response = page.waitForResponse(
    (response) => response.url().includes('/transcribe') && response.status() === 200
  );

  // Upload the MP3 file
  const filePath = '../test/fixtures/short.mp3';
  const fileInput = await page.locator('input[type="file"]');
  await fileInput.setInputFiles(filePath);

  await response;

  const result = await page.locator('#transcription');
  const text = await result.textContent();
  expect(text).toContain("The Blind Man's World");
  // Assert that some transcription text is present
  expect(text && text.length > 0).toBeTruthy();
});
