import { describe, expect, it } from 'vitest';
import { jsonErrorLine, validateJson, validateYaml } from './fileValidate';

describe('fileValidate', () => {
  it('accepts valid JSON', () => {
    expect(validateJson('{"a": 1}')).toBeUndefined();
  });

  it('reports JSON syntax errors with line numbers', () => {
    const text = '{\n  "a": 1,\n}';
    const result = validateJson(text);
    expect(result?.message).toBeTruthy();
    expect(result?.line).toBe(3);
  });

  it('maps JSON position to line', () => {
    const text = '{"broken": }';
    const err = new SyntaxError('Unexpected token } in JSON at position 11');
    expect(jsonErrorLine(text, err)).toBe(1);
  });

  it('accepts valid YAML', () => {
    expect(validateYaml('key: value\nlist:\n  - one\n')).toBeUndefined();
  });

  it('reports YAML parse errors with line numbers', () => {
    const text = 'key: value\n  bad: indent';
    const result = validateYaml(text);
    expect(result?.message).toBeTruthy();
    expect(result?.line).toBeGreaterThan(0);
  });

  it('rejects invalid YAML structure', () => {
    const text = 'key: [unclosed';
    const result = validateYaml(text);
    expect(result?.message).toBeTruthy();
  });
});
