// Reusable presentational primitives.
import type { ComponentChildren } from 'preact';
import { useEffect, useRef } from 'preact/hooks';
import type { AsyncState } from '../lib/hooks';
import { useStore } from '../lib/store';
import { IconClose } from './Icons';

export function Spinner() {
  return <output class="spinner" aria-label="Loading" />;
}

export function Loading({ label = 'Loading…' }: Readonly<{ label?: string }>) {
  return (
    <div class="loading">
      <Spinner />
      <span>{label}</span>
    </div>
  );
}

export function EmptyState({
  title,
  hint,
  children,
}: Readonly<{ title: string; hint?: string; children?: ComponentChildren }>) {
  return (
    <div class="empty">
      <strong>{title}</strong>
      {hint && <span>{hint}</span>}
      {children}
    </div>
  );
}

export function ErrorState({
  error,
  onRetry,
  message,
}: Readonly<{ error: Error; onRetry?: () => void; message?: string }>) {
  const msg = message ?? error.message;
  return (
    <div class="error-box">
      <strong>Something went wrong</strong>
      <span>{msg}</span>
      {onRetry && (
        <button type="button" class="btn sm" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}

/** Renders loading/error/empty/data states for an async resource. */
export function AsyncView<T>({
  state,
  children,
  isEmpty,
  empty,
  loadingLabel,
}: Readonly<{
  state: AsyncState<T>;
  children: (data: T) => ComponentChildren;
  isEmpty?: (data: T) => boolean;
  empty?: ComponentChildren;
  loadingLabel?: string;
}>) {
  if (state.loading && state.data === undefined) return <Loading label={loadingLabel} />;
  if (state.error) return <ErrorState error={state.error} onRetry={state.reload} />;
  if (state.data === undefined) return <Loading label={loadingLabel} />;
  if (isEmpty?.(state.data)) {
    return <>{empty ?? <EmptyState title="Nothing here yet" />}</>;
  }
  return <>{children(state.data)}</>;
}

export function Field({
  label,
  hint,
  error,
  children,
}: Readonly<{ label: string; hint?: string; error?: string; children: ComponentChildren }>) {
  return (
    <label class="field">
      <span>{label}</span>
      {children}
      {hint && <span class="hint">{hint}</span>}
      {error && <span class="field-error">{error}</span>}
    </label>
  );
}

export function InlineNotice({
  kind = 'info',
  children,
}: Readonly<{ kind?: 'info' | 'ok' | 'danger'; children: ComponentChildren }>) {
  return <p class={`inline-notice ${kind}`}>{children}</p>;
}

export function Badge({
  kind = 'off',
  children,
}: Readonly<{ kind?: 'ok' | 'off' | 'warn' | 'danger' | 'info'; children: ComponentChildren }>) {
  return (
    <span class={`badge ${kind}`}>
      <span class="dot" />
      {children}
    </span>
  );
}

export function Modal({
  title,
  onClose,
  children,
  footer,
  wide,
}: Readonly<{
  title: string;
  onClose: () => void;
  children: ComponentChildren;
  footer?: ComponentChildren;
  wide?: boolean;
}>) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dlg = ref.current;
    if (dlg && !dlg.open) dlg.showModal();
  }, []);

  return (
    <dialog
      ref={ref}
      class={`modal ${wide ? 'lg' : ''}`}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      <div class="modal-head">
        <h3>{title}</h3>
        <button type="button" class="btn ghost sm" aria-label="Close" onClick={onClose}>
          <IconClose width={16} height={16} />
        </button>
      </div>
      <div class="modal-body">{children}</div>
      {footer && <div class="modal-foot">{footer}</div>}
    </dialog>
  );
}

export function Toasts() {
  const { toasts, dismissToast } = useStore();
  if (toasts.length === 0) return null;
  return (
    <div class="toasts">
      {toasts.map((t) => (
        <button
          type="button"
          key={t.id}
          class={`toast ${t.kind === 'error' ? 'error' : ''}`}
          onClick={() => dismissToast(t.id)}
        >
          {t.message}
        </button>
      ))}
    </div>
  );
}

/** Section heading with optional kicker label. */
export function SectionTitle({
  title,
  description,
  actions,
}: Readonly<{ title: string; description?: string; actions?: ComponentChildren }>) {
  return (
    <div class="section-title">
      <div>
        <h2>{title}</h2>
        {description && <p>{description}</p>}
      </div>
      {actions && <div class="row">{actions}</div>}
    </div>
  );
}

/** Read-only key/value row. */
export function KeyValue({
  label,
  children,
  mono,
}: Readonly<{ label: string; children: ComponentChildren; mono?: boolean }>) {
  return (
    <div class="kv">
      <div class="kv-label">{label}</div>
      <div class={`kv-value ${mono ? 'mono' : ''}`}>{children}</div>
    </div>
  );
}
