// Splunk-style search bar for the /logs view.
// - Splunk "Smart Mode" `>` prompt button (also clears the query when clicked).
// - Compact source dropdown + monospace query input + time range + Run/Stop button.
// - Sub-bar: History/Live Tail mode toggle, select2-style Saved dropdown,
//   Raw/Smart format toggle.
//
// The component is a controlled view: it owns nothing except transient UI state
// (popover open, rename draft, etc). All persistent state (query, mode, format,
// selected saved query, etc.) lives in the parent LogsView and is passed in.

import type { JSX } from 'preact';
import { useEffect, useRef, useState } from 'preact/hooks';
import type { QueryOption, QuerySourceMeta, SavedQuery, SyntaxTopic } from '../api/types';
import { IconBookmark, IconPause, IconPencil, IconPlay, IconTrash } from './Icons';

export type LogMode = 'history' | 'live';
export type LogFormat = 'raw' | 'smart';

export interface Props {
  source: QuerySourceMeta | undefined;
  sources: QuerySourceMeta[];
  onSourceChange: (s: QuerySourceMeta) => void;
  query: string;
  onQueryChange: (q: string) => void;
  timeRange: string;
  timeRanges: QueryOption[];
  onTimeRangeChange: (r: string) => void;
  mode: LogMode;
  onModeChange: (m: LogMode) => void;
  running: boolean;
  onRun: () => void;
  onStop: () => void;
  saved: SavedQuery[];
  onSaveCurrent: (name: string) => Promise<void> | void;
  onLoadSaved: (s: SavedQuery) => void;
  onRenameSaved: (id: number, name: string) => Promise<void> | void;
  onDeleteSaved: (id: number) => Promise<void> | void;
  format: LogFormat;
  onFormatChange: (f: LogFormat) => void;
  syntaxHint: string;
  syntaxTopics: SyntaxTopic[];
  helpUrl?: string;
}

export function LogsSearch({
  source,
  sources,
  onSourceChange,
  query,
  onQueryChange,
  timeRange,
  timeRanges,
  onTimeRangeChange,
  mode,
  onModeChange,
  running,
  onRun,
  onStop,
  saved,
  onSaveCurrent,
  onLoadSaved,
  onRenameSaved,
  onDeleteSaved,
  format,
  onFormatChange,
  syntaxHint,
  syntaxTopics,
  helpUrl,
}: Readonly<Props>) {
  const [helpOpen, setHelpOpen] = useState(false);
  const helpRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!helpOpen) return;
    const onDocClick = (event: MouseEvent) => {
      if (helpRef.current && !helpRef.current.contains(event.target as Node)) {
        setHelpOpen(false);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setHelpOpen(false);
    };
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [helpOpen]);

  const handleRun = () => {
    if (running) onStop();
    else onRun();
  };

  const onPromptClick = () => {
    onQueryChange('');
  };

  const onInputKeyDown = (event: JSX.TargetedKeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      handleRun();
    }
  };

  const quickFilters = source?.quick_filters ?? [];
  const examples = source?.examples ?? [];

  return (
    <div class="splunk-bar" role="search" aria-label="Logs search">
      <div class="splunk-bar-row">
        <button
          type="button"
          class={`splunk-prompt${query ? ' active' : ''}`}
          aria-label="Smart mode (clears query)"
          title="Smart mode — clears the query"
          onClick={onPromptClick}
        >
          &gt;
        </button>
        <select
          class="splunk-source"
          aria-label="Source"
          value={source?.id ?? ''}
          onChange={(event) => {
            const next = sources.find((s) => s.id === (event.currentTarget as HTMLSelectElement).value);
            if (next) onSourceChange(next);
          }}
        >
          {sources.length === 0 && <option value="">source…</option>}
          {sources.map((item) => (
            <option key={item.id} value={item.id}>
              {item.label}
            </option>
          ))}
        </select>
        <input
          class="splunk-input"
          type="text"
          autoComplete="off"
          spellcheck={false}
          placeholder="status>=300 status<400 | head 50"
          value={query}
          onInput={(event) => onQueryChange((event.currentTarget as HTMLInputElement).value)}
          onKeyDown={onInputKeyDown}
        />
        <select
          class="splunk-time"
          aria-label="Time range"
          value={timeRange}
          onChange={(event) => onTimeRangeChange((event.currentTarget as HTMLSelectElement).value)}
        >
          {timeRanges.length === 0 && <option value="">range…</option>}
          {timeRanges.map((item) => (
            <option key={String(item.value)} value={String(item.value ?? '')}>
              {String(item.label ?? item.value ?? '')}
            </option>
          ))}
        </select>
        <button
          type="button"
          class={running ? 'splunk-stop' : 'splunk-run'}
          onClick={handleRun}
          aria-pressed={running}
        >
          {running ? (
            <>
              <IconPause width={14} height={14} /> Stop
            </>
          ) : (
            <>
              <IconPlay width={14} height={14} /> Run
            </>
          )}
        </button>
      </div>
      <div class="splunk-syntax-row">
        <div class="splunk-syntax">
          <code>{syntaxHint}</code>
        </div>
        <div ref={helpRef} class={`splunk-help${helpOpen ? ' open' : ''}`}>
          <button
            type="button"
            class="splunk-help-trigger"
            aria-label="Search syntax help"
            aria-expanded={helpOpen}
            title="Search syntax help"
            onClick={() => setHelpOpen((open) => !open)}
          >
            ?
          </button>
          {helpOpen && (
            <div class="splunk-help-panel" role="dialog" aria-label="Search syntax">
              <div class="splunk-help-title">Search syntax</div>
              {syntaxTopics.map((topic) => (
                <div class="splunk-help-topic" key={topic.title}>
                  <div class="topic-title">{topic.title}</div>
                  <code class="topic-syntax">{topic.syntax}</code>
                  {topic.example && <code class="topic-example">{topic.example}</code>}
                  {topic.note && <div class="topic-note">{topic.note}</div>}
                </div>
              ))}
              {helpUrl && (
                <a class="splunk-help-link" href={helpUrl} target="_blank" rel="noopener noreferrer">
                  Full tutorial →
                </a>
              )}
            </div>
          )}
        </div>
      </div>
      {(quickFilters.length > 0 || examples.length > 0) && (
        <div class="splunk-chips">
          {quickFilters.map((chip) => (
            <button
              key={chip.value}
              type="button"
              class="splunk-chip quick"
              title={chip.value}
              onClick={() => onQueryChange(chip.value)}
            >
              {chip.label}
            </button>
          ))}
          {examples.map((example) => (
            <button
              key={example}
              type="button"
              class="splunk-chip example"
              title={example}
              onClick={() => onQueryChange(example)}
            >
              {example}
            </button>
          ))}
        </div>
      )}
      <div class="splunk-subbar">
        <div class="splunk-mode" role="tablist" aria-label="Search mode">
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'history'}
            class={mode === 'history' ? 'active' : ''}
            onClick={() => onModeChange('history')}
          >
            History
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'live'}
            class={`live${mode === 'live' ? ' active' : ''}`}
            onClick={() => onModeChange('live')}
          >
            Live Tail
          </button>
        </div>
        <SavedDropdown
          saved={saved}
          onSaveCurrent={onSaveCurrent}
          onLoadSaved={onLoadSaved}
          onRenameSaved={onRenameSaved}
          onDeleteSaved={onDeleteSaved}
        />
        <div class="grow" />
        <div class="splunk-format" role="group" aria-label="Display format">
          <button
            type="button"
            class={format === 'raw' ? 'active' : ''}
            aria-pressed={format === 'raw'}
            onClick={() => onFormatChange('raw')}
          >
            Raw
          </button>
          <button
            type="button"
            class={format === 'smart' ? 'active' : ''}
            aria-pressed={format === 'smart'}
            onClick={() => onFormatChange('smart')}
          >
            Smart
          </button>
        </div>
      </div>
    </div>
  );
}

interface SavedDropdownProps {
  saved: SavedQuery[];
  onSaveCurrent: (name: string) => Promise<void> | void;
  onLoadSaved: (s: SavedQuery) => void;
  onRenameSaved: (id: number, name: string) => Promise<void> | void;
  onDeleteSaved: (id: number) => Promise<void> | void;
}

function SavedDropdown({
  saved,
  onSaveCurrent,
  onLoadSaved,
  onRenameSaved,
  onDeleteSaved,
}: Readonly<SavedDropdownProps>) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const [renamingId, setRenamingId] = useState<number | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);
  const [activeSaved, setActiveSaved] = useState<SavedQuery | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const confirmDeleteRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDocClick = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false);
        setFilter('');
        setRenamingId(null);
        setConfirmDeleteId(null);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
        setFilter('');
        setRenamingId(null);
        setConfirmDeleteId(null);
      }
    };
    document.addEventListener('mousedown', onDocClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDocClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  useEffect(() => {
    if (confirmDeleteId !== null) {
      const id = globalThis.setTimeout(() => confirmDeleteRef.current?.focus(), 0);
      return () => globalThis.clearTimeout(id);
    }
    return undefined;
  }, [confirmDeleteId]);

  useEffect(() => {
    if (open) {
      const id = globalThis.setTimeout(() => inputRef.current?.focus(), 0);
      return () => globalThis.clearTimeout(id);
    }
    return undefined;
  }, [open]);

  const trimmed = filter.trim();
  const lower = trimmed.toLowerCase();
  const matches = lower
    ? saved.filter(
        (item) => item.name.toLowerCase().includes(lower) || item.query.toLowerCase().includes(lower),
      )
    : saved;
  const exactName = saved.some((item) => item.name.toLowerCase() === lower);
  const showSaveAs = trimmed.length > 0 && !exactName;

  const triggerLabel = activeSaved ? activeSaved.name : 'Saved';
  const triggerCount = saved.length;

  const handleSaveAs = async () => {
    if (!trimmed) return;
    await onSaveCurrent(trimmed);
    setFilter('');
  };

  const beginRename = (item: SavedQuery) => {
    setRenamingId(item.id);
    setRenameValue(item.name);
  };

  const commitRename = async (id: number) => {
    const value = renameValue.trim();
    if (value) await onRenameSaved(id, value);
    setRenamingId(null);
    setRenameValue('');
  };

  const cancelRename = () => {
    setRenamingId(null);
    setRenameValue('');
  };

  const handleDelete = (item: SavedQuery) => {
    if (confirmDeleteId === item.id) {
      void onDeleteSaved(item.id);
      if (activeSaved?.id === item.id) setActiveSaved(null);
      setConfirmDeleteId(null);
      return;
    }
    setConfirmDeleteId(item.id);
  };

  const cancelDelete = () => {
    setConfirmDeleteId(null);
  };

  const handleLoad = (item: SavedQuery) => {
    onLoadSaved(item);
    setActiveSaved(item);
    setOpen(false);
    setFilter('');
  };

  return (
    <div ref={rootRef} class={`saved-select${open ? ' open' : ''}`}>
      <button
        type="button"
        class="saved-select-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <IconBookmark width={14} height={14} />
        <span>{triggerLabel}</span>
        <span class="count">{triggerCount}</span>
        <span class="caret" aria-hidden="true" />
      </button>
      {open && (
        <div class="saved-popover" role="listbox" aria-label="Saved queries">
          <div class="saved-popover-input">
            <input
              ref={inputRef}
              type="text"
              placeholder="Search or name a new query…"
              value={filter}
              onInput={(event) => setFilter((event.currentTarget as HTMLInputElement).value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && showSaveAs) {
                  event.preventDefault();
                  void handleSaveAs();
                }
              }}
            />
          </div>
          <div class="saved-popover-list">
            {showSaveAs && (
              <div
                class="saved-popover-row save-as"
                role="option"
                aria-selected="false"
                onClick={() => void handleSaveAs()}
              >
                <div class="row-main">
                  <div class="row-name">Save current as “{trimmed}”</div>
                  <div class="row-meta">stores the current source + query</div>
                </div>
              </div>
            )}
            {matches.length === 0 && !showSaveAs && (
              <div class="saved-popover-empty">
                {saved.length === 0 ? 'No saved queries yet — type a name to save the current one.' : 'No matches.'}
              </div>
            )}
            {matches.map((item) =>
              renamingId === item.id ? (
                <div class="saved-popover-row" key={item.id}>
                  <input
                    class="rename-input"
                    autoFocus
                    value={renameValue}
                    onInput={(event) => setRenameValue((event.currentTarget as HTMLInputElement).value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') {
                        event.preventDefault();
                        void commitRename(item.id);
                      } else if (event.key === 'Escape') {
                        event.preventDefault();
                        cancelRename();
                      }
                    }}
                    onBlur={() => void commitRename(item.id)}
                  />
                </div>
              ) : confirmDeleteId === item.id ? (
                <div class="saved-popover-row confirm-delete" key={item.id} role="option" aria-selected="false">
                  <div class="row-main">
                    <div class="row-name">Delete “{item.name}”?</div>
                    <div class="row-meta">This saved query will be removed permanently.</div>
                  </div>
                  <div class="saved-popover-actions" onClick={(event) => event.stopPropagation()}>
                    <button
                      type="button"
                      class="btn ghost sm"
                      aria-label={`Cancel deleting ${item.name}`}
                      onClick={cancelDelete}
                    >
                      Cancel
                    </button>
                    <button
                      ref={confirmDeleteRef}
                      type="button"
                      class="btn danger sm"
                      aria-label={`Confirm delete ${item.name}`}
                      onClick={() => handleDelete(item)}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ) : (
                <div
                  class="saved-popover-row"
                  key={item.id}
                  role="option"
                  aria-selected={activeSaved?.id === item.id}
                  onClick={() => handleLoad(item)}
                >
                  <div class="row-main">
                    <div class="row-name">{item.name}</div>
                    <div class="row-meta">
                      <span class={`badge src-${item.source}`} style={`padding:0 6px;font-size:10px;background:var(--src-${item.source}, var(--surface-2));color:var(--text-on-accent);`}>
                        {item.source}
                      </span>
                      <span class="q-preview">{item.query || '—'}</span>
                    </div>
                  </div>
                  <div class="saved-popover-actions" onClick={(event) => event.stopPropagation()}>
                    <button
                      type="button"
                      title="Load"
                      aria-label={`Load ${item.name}`}
                      onClick={() => handleLoad(item)}
                    >
                      <IconPlay width={12} height={12} />
                    </button>
                    <button
                      type="button"
                      title="Rename"
                      aria-label={`Rename ${item.name}`}
                      onClick={() => beginRename(item)}
                    >
                      <IconPencil width={12} height={12} />
                    </button>
                    <button
                      type="button"
                      class="danger"
                      title="Delete"
                      aria-label={`Delete ${item.name}`}
                      onClick={() => handleDelete(item)}
                    >
                      <IconTrash width={12} height={12} />
                    </button>
                  </div>
                </div>
              ),
            )}
          </div>
        </div>
      )}
    </div>
  );
}
