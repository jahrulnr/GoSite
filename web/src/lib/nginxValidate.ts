import { nginx, websites } from '../api/endpoints';
import { nginxTestScope } from './editorLanguage';
import { humanizeError } from './errors';

async function runValidate(
  fn: () => Promise<unknown>,
  signal: AbortSignal,
): Promise<string | undefined> {
  try {
    await fn();
    if (signal.aborted) return undefined;
    return undefined;
  } catch (err) {
    if ((err as Error).name === 'AbortError') throw err;
    return humanizeError(err as Error);
  }
}

export function validateGlobalNginx(text: string, signal: AbortSignal) {
  return runValidate(() => nginx.test(text, 'global', signal), signal);
}

export function validateDefaultNginx(text: string, signal: AbortSignal) {
  return runValidate(() => nginx.test(text, 'default', signal), signal);
}

export function validateSiteNginx(siteId: number, text: string, signal: AbortSignal) {
  return runValidate(() => websites.testNginxConfig(siteId, text, signal), signal);
}

export function validateFileNginx(text: string, signal: AbortSignal) {
  return runValidate(() => nginx.test(text, nginxTestScope(text), signal), signal);
}
