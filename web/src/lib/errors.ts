import type { UiMetaResponse } from '../api/types';

/** User-facing API error text — no error codes or internal tokens. */
export function humanizeError(error: Error, meta?: UiMetaResponse): string {
  const msg = error.message;
  const lower = msg.toLowerCase();

  if (lower.includes('web root') || lower.includes('path outside')) {
    const hint = meta?.websites?.static_path_hint;
    return hint
      ? `Path must be inside your websites folder. Try: ${hint}`
      : 'Path must be inside your websites folder.';
  }
  if (lower.includes('device and dir')) {
    return 'Enter both device and mount directory before saving.';
  }
  if (lower.includes('invalid token') && lower.includes('login')) {
    return 'Try action:login or use the Sign-ins quick filter instead of plain "login".';
  }

  return msg.replace(/_/g, ' ');
}

export function humanizeValidation(
  reason: string | undefined,
  meta?: UiMetaResponse,
): { text: string; ok: boolean } {
  if (!reason) return { text: '', ok: false };
  if (reason === 'Valid') return { text: 'Looks good — you can save.', ok: true };
  const lower = reason.toLowerCase();
  if (lower.includes('web root') || lower.includes('path outside')) {
    const hint = meta?.websites?.static_path_hint;
    return {
      text: hint
        ? `Path must be inside your websites folder. Example: ${hint}`
        : 'Path must be inside your websites folder.',
      ok: false,
    };
  }
  return { text: humanizeError(new Error(reason), meta), ok: false };
}
