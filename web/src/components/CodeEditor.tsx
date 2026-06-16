import { useEffect, useRef } from 'preact/hooks';
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
import { linter, lintGutter, type Diagnostic } from '@codemirror/lint';
import { EditorState, type Extension } from '@codemirror/state';
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  placeholder,
} from '@codemirror/view';
import { languageExtension, type EditorLanguage } from '../lib/editorLanguage';
import { codeHighlighting, flattenExtensions } from '../lib/editorHighlight';
import { parseNginxErrorLine, shortenNginxError } from '../lib/nginxLint';
import {
  normalizeValidationResult,
  type CodeValidateFn,
  type ValidationResult,
} from '../lib/validation';

export type { CodeValidateFn, ValidationResult };

export interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  language?: EditorLanguage;
  placeholder?: string;
  className?: string;
  minHeight?: string;
  readOnly?: boolean;
  onValidate?: CodeValidateFn;
  validateDelayMs?: number;
}

function createTheme(minHeight?: string): Extension {
  return EditorView.theme({
    '&': {
      minHeight: minHeight ?? '220px',
    },
  });
}

function diagnosticFromResult(view: EditorView, result: ValidationResult): Diagnostic[] {
  const message = result.message.includes('[emerg]') ? shortenNginxError(result.message) : result.message;
  const lineNo = result.line ?? parseNginxErrorLine(result.message);
  if (lineNo && lineNo >= 1 && lineNo <= view.state.doc.lines) {
    const line = view.state.doc.line(lineNo);
    return [{ from: line.from, to: line.to, severity: 'error', message }];
  }
  const end = Math.min(view.state.doc.length, 1);
  return [{ from: 0, to: end, severity: 'error', message }];
}

function createRemoteLinter(validate: CodeValidateFn, delayMs: number) {
  let abort: AbortController | undefined;

  return linter(
    async (view): Promise<Diagnostic[]> => {
      abort?.abort();
      abort = new AbortController();
      const signal = abort.signal;
      const text = view.state.doc.toString();
      if (!text.trim()) return [];

      try {
        const raw = await validate(text, signal);
        const result = normalizeValidationResult(raw, parseNginxErrorLine);
        if (!result) return [];
        return diagnosticFromResult(view, result);
      } catch (err) {
        if ((err as Error).name === 'AbortError') return [];
        return [
          {
            from: 0,
            to: Math.min(1, view.state.doc.length),
            severity: 'error',
            message: (err as Error).message,
          },
        ];
      }
    },
    { delay: delayMs },
  );
}

export function CodeEditor({
  value,
  onChange,
  language = 'text',
  placeholder: placeholderText,
  className,
  minHeight,
  readOnly = false,
  onValidate,
  validateDelayMs = 800,
}: Readonly<CodeEditorProps>) {
  const hostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView>();
  const onChangeRef = useRef(onChange);
  const validateRef = useRef(onValidate);
  onChangeRef.current = onChange;
  validateRef.current = onValidate;

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const extensions: Extension[] = [
      lineNumbers(),
      highlightActiveLineGutter(),
      highlightActiveLine(),
      drawSelection(),
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.lineWrapping,
      EditorState.readOnly.of(readOnly),
      codeHighlighting(),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) onChangeRef.current(update.state.doc.toString());
      }),
      createTheme(minHeight),
    ];
    extensions.push(...flattenExtensions(languageExtension(language)));

    if (placeholderText) extensions.push(placeholder(placeholderText));
    if (validateRef.current) {
      extensions.push(
        lintGutter(),
        createRemoteLinter((text, signal) => validateRef.current!(text, signal), validateDelayMs),
      );
    }

    const view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions,
      }),
    });
    viewRef.current = view;

    return () => {
      view.destroy();
      viewRef.current = undefined;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [language, minHeight, placeholderText, validateDelayMs, readOnly]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== value) {
      view.dispatch({
        changes: { from: 0, to: current.length, insert: value },
      });
    }
  }, [value]);

  return <div ref={hostRef} class={`code-editor${className ? ` ${className}` : ''}`} />;
}
