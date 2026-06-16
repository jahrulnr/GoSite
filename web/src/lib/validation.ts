/** Shared editor validation result with optional 1-based line number. */
export type ValidationResult = { message: string; line?: number };

export type CodeValidateFn = (
  value: string,
  signal: AbortSignal,
) => Promise<ValidationResult | string | undefined>;

export function normalizeValidationResult(
  result: ValidationResult | string | undefined,
  fallbackLine?: (message: string) => number | undefined,
): ValidationResult | undefined {
  if (!result) return undefined;
  if (typeof result === 'string') {
    return { message: result, line: fallbackLine?.(result) };
  }
  return result;
}
