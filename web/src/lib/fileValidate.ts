import YAML, { YAMLParseError } from 'yaml';
import type { FileEntry } from '../api/types';
import {
  detectLanguage,
  isValidatableLanguage,
  languageDisplayName,
  type EditorLanguage,
  validationLabel,
} from './editorLanguage';
import { validateFileNginx } from './nginxValidate';
import { parseNginxErrorLine, shortenNginxError } from './nginxLint';
import type { CodeValidateFn, ValidationResult } from './validation';

export type FileDiagnostic = { kind: 'ok' | 'danger' | 'warn'; message: string };

export function jsonErrorLine(text: string, err: SyntaxError): number | undefined {
  const posMatch = err.message.match(/position (\d+)/i);
  if (!posMatch) return undefined;
  const pos = Number(posMatch[1]);
  if (!Number.isFinite(pos) || pos < 0) return undefined;

  let line = 1;
  for (let i = 0; i < pos && i < text.length; i++) {
    if (text.charCodeAt(i) === 10) line++;
  }
  return line;
}

export function validateJson(text: string): ValidationResult | undefined {
  if (!text.trim()) return undefined;
  try {
    JSON.parse(text);
    return undefined;
  } catch (err) {
    const error = err as SyntaxError;
    return { message: error.message, line: jsonErrorLine(text, error) };
  }
}

export function validateYaml(text: string): ValidationResult | undefined {
  if (!text.trim()) return undefined;
  try {
    YAML.parse(text, { prettyErrors: true });
    return undefined;
  } catch (err) {
    if (!(err instanceof YAMLParseError)) {
      return { message: (err as Error).message };
    }
    const line = err.linePos?.[0]?.line;
    const col = err.linePos?.[0]?.col;
    const where = line ? (col ? ` (line ${line}, column ${col})` : ` (line ${line})`) : '';
    return { message: `${err.message}${where}`, line };
  }
}

function validateSync(lang: EditorLanguage, text: string): ValidationResult | undefined {
  switch (lang) {
    case 'json':
      return validateJson(text);
    case 'yaml':
      return validateYaml(text);
    default:
      return undefined;
  }
}

export function lintDiagnosticForFile(entry: FileEntry, content: string): FileDiagnostic {
  const lang = detectLanguage(entry.path, entry, content);

  if (lang === 'json' || lang === 'yaml') {
    const issue = validateSync(lang, content);
    if (issue) return { kind: 'danger', message: issue.message };
    return { kind: 'ok', message: validationLabel(lang) };
  }

  if (lang === 'nginx') {
    return { kind: 'ok', message: 'Nginx syntax checked in editor.' };
  }

  if (lang !== 'text') {
    return { kind: 'ok', message: `${languageDisplayName(lang)} file ready.` };
  }

  if (entry.kind === 'config') {
    const unbalanced = (content.match(/\{/g)?.length ?? 0) !== (content.match(/\}/g)?.length ?? 0);
    return unbalanced
      ? { kind: 'warn', message: 'Braces look unbalanced.' }
      : { kind: 'ok', message: 'No obvious config issues.' };
  }

  if (isValidatableLanguage(lang)) {
    return { kind: 'ok', message: validationLabel(lang) };
  }

  return {
    kind: entry.editable ? 'ok' : 'warn',
    message: entry.editable ? 'Text file ready.' : 'Preview only.',
  };
}

export function validatorForLanguage(lang: EditorLanguage): CodeValidateFn | undefined {
  switch (lang) {
    case 'json':
      return async (text) => validateJson(text);
    case 'yaml':
      return async (text) => validateYaml(text);
    case 'nginx':
      return async (text, signal) => {
        const message = await validateFileNginx(text, signal);
        if (!message) return undefined;
        return {
          message: shortenNginxError(message),
          line: parseNginxErrorLine(message),
        };
      };
    default:
      return undefined;
  }
}

export function validatorForFile(
  entry: Pick<FileEntry, 'path' | 'kind' | 'extension'>,
  content?: string,
): CodeValidateFn | undefined {
  const lang = detectLanguage(entry.path, entry, content);
  return validatorForLanguage(lang);
}
