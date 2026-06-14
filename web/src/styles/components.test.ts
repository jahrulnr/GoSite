import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

describe('form control focus CSS', () => {
  it('does not use background shorthand on select focus', () => {
    const css = fs.readFileSync(path.join(process.cwd(), 'src/styles/components.css'), 'utf8');
    const focusRule = css.match(/\.input:focus, \.select:focus, \.textarea:focus \{[\s\S]*?\}/)?.[0] ?? '';
    expect(focusRule).toContain('background-color: var(--bg-elevated);');
    // Regression: `background:` shorthand resets background-repeat/position/size,
    // which made the select chevron SVG tile across the field while focused.
    expect(focusRule).not.toMatch(/\n\s*background:\s/);
  });

  it('keeps select chevron from repeating', () => {
    const css = fs.readFileSync(path.join(process.cwd(), 'src/styles/components.css'), 'utf8');
    const selectRule = css.match(/\.select \{[\s\S]*?\}/)?.[0] ?? '';
    expect(selectRule).toContain('background-repeat: no-repeat;');
    expect(selectRule).toContain('background-size: 14px;');
    expect(selectRule).toContain('background-position: right 0.85rem center;');
  });
});
