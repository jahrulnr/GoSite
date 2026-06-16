/** Parse nginx -t error output for a 1-based line number. */
export function parseNginxErrorLine(message: string): number | undefined {
  const inConf = message.match(/\.conf:(\d+)\b/);
  if (inConf) return Number(inConf[1]);

  const emerg = message.match(/\[emerg\][^\n]*:(\d+)\b/);
  if (emerg) return Number(emerg[1]);

  const trailing = message.match(/:(\d+)\s*$/m);
  if (trailing) return Number(trailing[1]);

  return undefined;
}

/** Keep the most useful single-line nginx error for lint tooltips. */
export function shortenNginxError(message: string): string {
  const lines = message
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
  const emerg = lines.find((line) => line.includes('[emerg]') || line.includes('[error]'));
  if (emerg) return emerg.replace(/^[\d/]+\s+\[\w+\]\s+\d+#\d+:\s*/, '');
  return lines[lines.length - 1] ?? message;
}
