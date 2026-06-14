import type { AuthMetadata, UiMetaResponse } from '../api/types';

export type FileRoot = { path: string; label?: string };

/** Auth metadata includes file_roots from GET /auth/login (snake_case). */
type AuthMetaWithRoots = AuthMetadata & {
  file_roots?: FileRoot[];
  web_root?: string;
};

/** Merge ui/meta with /auth/login metadata. Ensures files.roots exist even when
 * ui/meta was fetched before the session cookie existed (common after login). */
export function mergePanelMeta(
  uiMeta: UiMetaResponse | undefined,
  authMeta: AuthMetaWithRoots | undefined,
): UiMetaResponse | undefined {
  if (!uiMeta && !authMeta) return undefined;
  const merged: UiMetaResponse = {
    ...(uiMeta ?? ({} as UiMetaResponse)),
    auth: {
      login_hint: uiMeta?.auth?.login_hint ?? '',
      remember_me: uiMeta?.auth?.remember_me ?? false,
      basic_auth_enabled: authMeta?.basic_auth_enabled ?? uiMeta?.auth?.basic_auth_enabled ?? false,
      lockscreen_enabled: authMeta?.lockscreen_enabled ?? uiMeta?.auth?.lockscreen_enabled ?? false,
      lock_after_seconds: authMeta?.lock_after_seconds ?? uiMeta?.auth?.lock_after_seconds ?? 0,
      ...authMeta,
    },
  };
  if (!merged.files?.roots?.length && authMeta?.file_roots?.length) {
    merged.files = {
      roots: authMeta.file_roots,
      actions: merged.files?.actions ?? [],
    };
  }
  return merged;
}

/** Resolve browsable roots from either files.roots or legacy auth.file_roots. */
export function fileRootsFromMeta(meta: UiMetaResponse | undefined): FileRoot[] {
  if (!meta) return [];
  if (meta.files?.roots?.length) return meta.files.roots;
  const legacy = (meta.auth as AuthMetaWithRoots | undefined)?.file_roots;
  return legacy ?? [];
}
