// Vitest setup: ensure DOM is reset between tests.
import { afterEach, vi, beforeAll } from 'vitest';
import { cleanup } from '@testing-library/preact';

beforeAll(() => {
  // jsdom does not implement <dialog>.showModal(). Without this, every Modal
  // throws on mount and downstream assertions can't query the rendered DOM.
  if (typeof HTMLDialogElement !== 'undefined') {
    HTMLDialogElement.prototype.showModal = function () {};
    HTMLDialogElement.prototype.close = function () {};
  }
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
