import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';
import type { Extension } from '@codemirror/state';

/** Dark-theme token colors for CodeMirror legacy + Lezer modes. */
export const codeHighlightStyle = HighlightStyle.define([
  { tag: tags.tagName, color: '#7ee787' },
  { tag: tags.attributeName, color: '#79c0ff' },
  { tag: tags.attributeValue, color: '#ce9178' },
  { tag: tags.string, color: '#ce9178' },
  { tag: tags.comment, color: '#8b949e', fontStyle: 'italic' },
  { tag: tags.keyword, color: '#ff7b72' },
  { tag: tags.propertyName, color: '#79c0ff' },
  { tag: tags.definition(tags.propertyName), color: '#79c0ff' },
  { tag: tags.typeName, color: '#ffa657' },
  { tag: tags.className, color: '#ffa657' },
  { tag: tags.number, color: '#79c0ff' },
  { tag: tags.bool, color: '#79c0ff' },
  { tag: tags.operator, color: '#d2a8ff' },
  { tag: tags.invalid, color: '#f85149' },
  { tag: tags.meta, color: '#8b949e' },
  { tag: tags.heading, color: '#79c0ff', fontWeight: 'bold' },
  { tag: tags.link, color: '#58a6ff', textDecoration: 'underline' },
  { tag: tags.variableName, color: '#c9d1d9' },
  { tag: tags.definition(tags.variableName), color: '#d2a8ff' },
]);

export function codeHighlighting(): Extension {
  return syntaxHighlighting(codeHighlightStyle);
}

export function flattenExtensions(ext: Extension | Extension[] | undefined): Extension[] {
  if (!ext) return [];
  if (Array.isArray(ext)) return ext.flatMap((item) => flattenExtensions(item));
  if (typeof ext === 'object' && ext !== null && 'extension' in ext) {
    const nested = (ext as { extension: Extension | Extension[] }).extension;
    // Some Extension values (e.g., FacetProvider) expose themselves as .extension.
    // Unwrapping those would recurse forever; treat them as leaf extensions.
    if (nested === ext) return [ext];
    return flattenExtensions(nested);
  }
  return [ext];
}
