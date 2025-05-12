# E2E Tests for talk-tailor

## Overview

This directory contains end-to-end tests using [Playwright](https://playwright.dev/) and [testcontainers](https://www.testcontainers.org/). The main test spins up the Docker container for the app, uploads an MP3 file, and verifies transcription via the web UI.

## Prerequisites

- Docker installed and running
- Node.js and npm
- `OPENAI_API_KEY` set in your environment

## Running the E2E Test

```sh
cd client
OPENAI_API_KEY=your-key npx playwright test tests/e2e/transcription.spec.ts
```

## Test Details

- The test uses `@testcontainers/docker` to start the `talk-tailor:latest` Docker image.
- The frontend and backend are served from the same container.
- The test uploads `../../test/fixtures/short.mp3` via the UI and checks for a transcription result.

## Troubleshooting

- Ensure the Docker image is built and tagged as `talk-tailor:latest`.
- The MP3 file path is relative to the test file; adjust if your directory structure changes.
