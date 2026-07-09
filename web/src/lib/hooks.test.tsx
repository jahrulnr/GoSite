import { fireEvent, render, screen, waitFor } from '@testing-library/preact';
import { describe, expect, it, vi } from 'vitest';
import { useState } from 'preact/hooks';
import { useAction, useAsync } from './hooks';

function AsyncProbe({ loader }: Readonly<{ loader: () => Promise<string> }>) {
  const state = useAsync(loader);
  if (state.loading && state.data === undefined) return <div>loading</div>;
  return (
    <div>
      <span data-testid="value">{state.data}</span>
      <button type="button" onClick={state.reload}>
        Reload
      </button>
    </div>
  );
}

function ActionProbe({ onSubmit }: Readonly<{ onSubmit: (value: string) => Promise<void> }>) {
  const [value, setValue] = useState('initial');
  const action = useAction(() => onSubmit(value));

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void action.run();
      }}
    >
      <input aria-label="value" value={value} onInput={(event) => setValue((event.target as HTMLInputElement).value)} />
      <button type="submit">Save</button>
    </form>
  );
}

describe('useAction', () => {
  it('runs the latest callback instead of the first-render closure', async () => {
    const onSubmit = vi.fn(async () => {});
    render(<ActionProbe onSubmit={onSubmit} />);

    fireEvent.input(screen.getByLabelText('value'), { target: { value: 'updated' } });
    fireEvent.click(screen.getByText('Save'));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('updated'));
  });
});

describe('useAsync', () => {
  it('keeps stale data visible while reloading', async () => {
    let resolve!: (value: string) => void;
    const loader = vi.fn(
      () =>
        new Promise<string>((res) => {
          resolve = res;
        }),
    );
    render(<AsyncProbe loader={loader} />);

    expect(screen.getByText('loading')).toBeTruthy();
    resolve('first');
    await waitFor(() => expect(screen.getByTestId('value').textContent).toBe('first'));

    fireEvent.click(screen.getByText('Reload'));
    expect(screen.queryByText('loading')).toBeNull();
    expect(screen.getByTestId('value').textContent).toBe('first');

    resolve('second');
    await waitFor(() => expect(screen.getByTestId('value').textContent).toBe('second'));
    expect(loader).toHaveBeenCalledTimes(2);
  });
});
