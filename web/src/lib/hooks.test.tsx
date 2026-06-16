import { fireEvent, render, screen, waitFor } from '@testing-library/preact';
import { describe, expect, it, vi } from 'vitest';
import { useState } from 'preact/hooks';
import { useAction } from './hooks';

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
