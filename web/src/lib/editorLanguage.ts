import { json } from '@codemirror/lang-json';
import { LanguageSupport, StreamLanguage } from '@codemirror/language';
import { css } from '@codemirror/legacy-modes/mode/css';
import { dockerFile } from '@codemirror/legacy-modes/mode/dockerfile';
import { javascript } from '@codemirror/legacy-modes/mode/javascript';
import { nginx } from '@codemirror/legacy-modes/mode/nginx';
import { properties } from '@codemirror/legacy-modes/mode/properties';
import { shell } from '@codemirror/legacy-modes/mode/shell';
import { toml } from '@codemirror/legacy-modes/mode/toml';
import { xml, html } from '@codemirror/legacy-modes/mode/xml';
import { yaml } from '@codemirror/legacy-modes/mode/yaml';
import type { Extension } from '@codemirror/state';

export type EditorLanguage =
  | 'nginx'
  | 'shell'
  | 'json'
  | 'yaml'
  | 'javascript'
  | 'css'
  | 'html'
  | 'xml'
  | 'toml'
  | 'dockerfile'
  | 'properties'
  | 'text';

export interface FileLanguageHint {
  kind?: string;
  extension?: string;
}

const EXTENSION_LANGUAGE: Record<string, EditorLanguage> = {
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  sh: 'shell',
  bash: 'shell',
  css: 'css',
  js: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  ts: 'javascript',
  tsx: 'javascript',
  jsx: 'javascript',
  xml: 'xml',
  svg: 'xml',
  html: 'html',
  htm: 'html',
  xhtml: 'html',
  toml: 'toml',
  ini: 'properties',
  cnf: 'properties',
  properties: 'properties',
  env: 'properties',
};

const KIND_LANGUAGE: Record<string, EditorLanguage> = {
  json: 'json',
  yaml: 'yaml',
};

function fileNameFromPath(path: string): string {
  return path.split('/').pop()?.toLowerCase() ?? '';
}

function extensionFromPath(path: string): string {
  const name = fileNameFromPath(path);
  if (name === 'dockerfile' || name === 'containerfile') return 'dockerfile';
  if (!name.includes('.')) return '';
  return name.split('.').pop() ?? '';
}

function isNginxPath(path: string, ext: string): boolean {
  const name = fileNameFromPath(path);
  return (
    ext === 'conf' ||
    name.includes('nginx') ||
    path.includes('/site.d/') ||
    path.includes('/active.d/') ||
    path.includes('/http.d/')
  );
}

function languageFromExtension(ext: string): EditorLanguage | undefined {
  if (!ext) return undefined;
  return EXTENSION_LANGUAGE[ext.toLowerCase()];
}

function languageFromPath(path: string): EditorLanguage {
  const ext = extensionFromPath(path);
  if (ext === 'dockerfile') return 'dockerfile';
  if (isNginxPath(path, ext)) return 'nginx';
  return languageFromExtension(ext) ?? 'text';
}

/** Sniff language from file content when path/extension are ambiguous. */
export function languageFromContent(content: string): EditorLanguage | undefined {
  const trimmed = content.trimStart();
  if (!trimmed) return undefined;

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) return 'json';
  if (trimmed.startsWith('---') || /^[\w.-]+\s*:/m.test(trimmed)) return 'yaml';
  if (/^(user|worker_processes|events|http)\b/m.test(trimmed) || /^server\s*\{/m.test(trimmed)) {
    return 'nginx';
  }
  if (/^<!DOCTYPE\s+html/i.test(trimmed) || /^<html[\s>]/i.test(trimmed)) return 'html';
  if (trimmed.startsWith('<?xml')) return 'xml';
  if (trimmed.startsWith('<')) return 'html';
  if (/^FROM\s+\S/m.test(trimmed)) return 'dockerfile';

  return undefined;
}

export function detectLanguage(path: string, hint?: FileLanguageHint, content?: string): EditorLanguage {
  const kind = hint?.kind?.toLowerCase();
  if (kind && KIND_LANGUAGE[kind]) return KIND_LANGUAGE[kind];

  const hintExt = hint?.extension?.toLowerCase();
  if (hintExt) {
    if (hintExt === 'dockerfile') return 'dockerfile';
    if (isNginxPath(path, hintExt)) return 'nginx';
    const fromHint = languageFromExtension(hintExt);
    if (fromHint) return fromHint;
  }

  const fromPath = languageFromPath(path);
  if (fromPath !== 'text') return fromPath;

  if (content) {
    const fromContent = languageFromContent(content);
    if (fromContent) return fromContent;
  }

  return 'text';
}

export function languageForPath(path: string): EditorLanguage {
  return detectLanguage(path);
}

export function languageDisplayName(lang: EditorLanguage): string {
  switch (lang) {
    case 'html':
      return 'HTML';
    case 'xml':
      return 'XML';
    case 'javascript':
      return 'JavaScript';
    case 'css':
      return 'CSS';
    case 'json':
      return 'JSON';
    case 'yaml':
      return 'YAML';
    case 'nginx':
      return 'Nginx';
    case 'shell':
      return 'Shell';
    case 'toml':
      return 'TOML';
    case 'dockerfile':
      return 'Dockerfile';
    case 'properties':
      return 'Properties';
    default:
      return 'Text';
  }
}

export function displayKind(entry: FileLanguageHint & { path: string; kind: string }): string {
  if (entry.kind === 'image' || entry.kind === 'directory' || entry.kind === 'archive' || entry.kind === 'binary') {
    return entry.kind;
  }
  const lang = detectLanguage(entry.path, entry);
  if (lang !== 'text') return languageDisplayName(lang).toLowerCase();
  return entry.kind || 'text';
}

function streamLanguageSupport(parser: Parameters<typeof StreamLanguage.define>[0]): LanguageSupport {
  return new LanguageSupport(StreamLanguage.define(parser));
}

export function languageExtension(lang: EditorLanguage): Extension | Extension[] {
  switch (lang) {
    case 'nginx':
      return streamLanguageSupport(nginx).extension;
    case 'shell':
      return streamLanguageSupport(shell).extension;
    case 'json':
      return json().extension;
    case 'yaml':
      return streamLanguageSupport(yaml).extension;
    case 'javascript':
      return streamLanguageSupport(javascript).extension;
    case 'css':
      return streamLanguageSupport(css).extension;
    case 'html':
      return streamLanguageSupport(html).extension;
    case 'xml':
      return streamLanguageSupport(xml).extension;
    case 'toml':
      return streamLanguageSupport(toml).extension;
    case 'dockerfile':
      return streamLanguageSupport(dockerFile).extension;
    case 'properties':
      return streamLanguageSupport(properties).extension;
    default:
      return [];
  }
}

/** Pick nginx test scope from config shape (server block vs full nginx.conf). */
export function nginxTestScope(text: string): 'default' | 'global' {
  const trimmed = text.trimStart();
  if (/^(user|worker_processes|events|http)\b/m.test(trimmed)) return 'global';
  return 'default';
}

export function isNginxLikePath(path: string): boolean {
  return detectLanguage(path) === 'nginx';
}

export function isValidatableLanguage(lang: EditorLanguage): boolean {
  return lang === 'json' || lang === 'yaml' || lang === 'nginx';
}

export function validationLabel(lang: EditorLanguage): string {
  switch (lang) {
    case 'json':
      return 'JSON valid.';
    case 'yaml':
      return 'YAML valid.';
    case 'nginx':
      return 'Nginx config valid.';
    default:
      return 'No validation for this file type.';
  }
}
