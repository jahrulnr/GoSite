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
  if (lower.includes('auth_token_expired') || lower.includes('auth token expired')) {
    return 'GitHub or GitLab token was rejected. Set GITHUB_TOKEN on the host (Settings → Plugins) or install via Artifact upload.';
  }
  if (lower.includes('resolve_stale') || lower.includes('resolve stale')) {
    return 'Release changed since preview — click Resolve again.';
  }
  if (lower.includes('resolve_failed') || lower.includes('resolve failed')) {
    return 'Could not find that repo, tag, or asset. Public repos need no token; for private repos set GITHUB_TOKEN on the host or use Artifact upload.';
  }
  if (lower.includes('fetch_digest_mismatch') || lower.includes('digest mismatch')) {
    return 'Downloaded file does not match the expected SHA-256.';
  }
  if (lower.includes('release_integrity')) {
    return 'Release asset changed since the index was published. Contact the vendor or use a new tag.';
  }
  if (lower.includes('platform_unsupported') || lower.includes('platform unsupported')) {
    return 'No build for this server OS/architecture.';
  }
  if (lower.includes('remote_install_disabled') || lower.includes('remote install disabled')) {
    return 'Remote install is disabled on this host. Use Artifact or Manifest JSON upload.';
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
