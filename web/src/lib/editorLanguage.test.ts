import { describe, expect, it } from 'vitest';
import { detectLanguage, displayKind, languageForPath, languageFromContent, languageDisplayName, nginxTestScope } from './editorLanguage';
import { parseNginxErrorLine, shortenNginxError } from './nginxLint';

describe('editorLanguage', () => {
  it('detects nginx paths', () => {
    expect(languageForPath('/storage/webconfig/site.d/example.com.conf')).toBe('nginx');
    expect(languageForPath('/etc/nginx/http.d/default.conf')).toBe('nginx');
    expect(languageForPath('/www/default/index.html')).toBe('html');
    expect(languageForPath('/www/default/page.htm')).toBe('html');
    expect(languageForPath('/www/default/data.xml')).toBe('xml');
  });

  it('detects json and yaml from extension', () => {
    expect(detectLanguage('/data/config.json')).toBe('json');
    expect(detectLanguage('/data/docker-compose.yml')).toBe('yaml');
    expect(detectLanguage('/data/docker-compose.yaml')).toBe('yaml');
  });

  it('detects shell, css, toml, dockerfile', () => {
    expect(detectLanguage('/scripts/deploy.sh')).toBe('shell');
    expect(detectLanguage('/www/assets/site.css')).toBe('css');
    expect(detectLanguage('/config/app.toml')).toBe('toml');
    expect(detectLanguage('/app/Dockerfile')).toBe('dockerfile');
  });

  it('prefers backend kind over ambiguous path', () => {
    expect(detectLanguage('/tmp/data', { kind: 'json', extension: '' }, '{"a":1}')).toBe('json');
    expect(detectLanguage('/tmp/data', { kind: 'yaml', extension: '' }, 'key: value')).toBe('yaml');
  });

  it('sniffs content when path is ambiguous', () => {
    expect(languageFromContent('{"hello": true}')).toBe('json');
    expect(languageFromContent('---\nkey: value')).toBe('yaml');
    expect(languageFromContent('<!DOCTYPE html>\n<html><body></body></html>')).toBe('html');
    expect(languageFromContent('<?xml version="1.0"?><root/>')).toBe('xml');
    expect(languageFromContent('server {\n  listen 80;\n}')).toBe('nginx');
  });

  it('shows html kind from extension', () => {
    expect(displayKind({ path: '/www/default/index.html', kind: 'text', extension: 'html' })).toBe('html');
    expect(languageDisplayName('html')).toBe('HTML');
  });

  it('picks nginx test scope from config shape', () => {
    expect(nginxTestScope('server { listen 80; }')).toBe('default');
    expect(nginxTestScope('user apps;\nevents {}')).toBe('global');
  });
});

describe('nginxLint', () => {
  it('parses line numbers from nginx errors', () => {
    const msg = '2024/06/16 [emerg] "server" directive is not allowed here in /tmp/nginx-raw.conf:1';
    expect(parseNginxErrorLine(msg)).toBe(1);
  });

  it('shortens emerg messages', () => {
    const msg = '2024/06/16 03:08:24 [emerg] 80#80: "server" directive is not allowed here in /tmp/x.conf:1';
    expect(shortenNginxError(msg)).toContain('server');
  });
});
