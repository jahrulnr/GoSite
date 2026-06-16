// Unit tests for the keystroke buffering in TerminalClient.
//
// The original client dropped any keystroke that arrived before the
// server's `ready` frame had been processed (currentRole was still
// 'none'). The fix queues those keystrokes in `pendingInput` and
// flushes them once both the socket is open AND the role is 'writer'.
// These tests cover the queue, the flush triggers, and the no-drop
// invariant for typing-while-connecting.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

class FakeWebSocket {
  static readonly instances: FakeWebSocket[] = [];
  static readonly OPEN = 1;
  static readonly CONNECTING = 0;
  static readonly CLOSED = 3;

  readyState = FakeWebSocket.CONNECTING;
  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string | ArrayBuffer }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(_url: string) {
    FakeWebSocket.instances.push(this);
  }

  send(data: string) {
    this.sent.push(data);
  }

  close() {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.();
  }

  // Simulate the server-side handshake.
  openAsWriter() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
    this.deliver(
      JSON.stringify({
        type: 'ready',
        session_id: 'sess-1',
        shell: '/bin/sh',
        cwd: '/tmp',
        cols: 80,
        rows: 24,
        role: 'writer',
        buffered_bytes: 0,
        first_seq: 0,
        end_seq: 0,
        sticky_ttl: '12h',
        started_at: '2025-01-01T00:00:00Z',
      }),
    );
  }

  openAsReader() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
    this.deliver(
      JSON.stringify({
        type: 'ready',
        session_id: 'sess-1',
        shell: '/bin/sh',
        cwd: '/tmp',
        cols: 80,
        rows: 24,
        role: 'reader',
        buffered_bytes: 0,
        first_seq: 0,
        end_seq: 0,
        sticky_ttl: '12h',
        started_at: '2025-01-01T00:00:00Z',
      }),
    );
  }

  deliver(text: string) {
    this.onmessage?.({ data: text });
  }
}

let origWS: typeof WebSocket;

beforeEach(() => {
  FakeWebSocket.instances = [];
  origWS = globalThis.WebSocket;
  // @ts-expect-error - test stub
  globalThis.WebSocket = FakeWebSocket;
});

afterEach(() => {
  globalThis.WebSocket = origWS;
  vi.resetModules();
});

describe('TerminalClient input buffering', () => {
  it('queues keystrokes typed before the socket opens', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onConnectionChange = vi.fn();
    const client = new TerminalClient({
      onData: () => {},
      onText: () => {},
      onRole: () => {},
      onConnectionChange,
      onError: () => {},
    });
    client.start();

    // Type before the WS has even opened.
    const enc = new TextEncoder();
    expect(client.sendInput(enc.encode('h'))).toBe(true);
    expect(client.sendInput(enc.encode('i'))).toBe(true);
    expect(FakeWebSocket.instances[0].sent).toEqual([]);
  });

  it('does not flush buffered keystrokes on socket open alone (role is still unknown)', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const client = new TerminalClient({
      onData: () => {},
      onText: () => {},
      onRole: () => {},
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();

    const enc = new TextEncoder();
    client.sendInput(enc.encode('h'));

    // Simulate just the TCP/TLS open, before the server's `ready`
    // frame. flushPendingInput must short-circuit because the role is
    // still 'none'.
    const ws = FakeWebSocket.instances[0];
    ws.readyState = FakeWebSocket.OPEN;
    ws.onopen?.();
    expect(ws.sent).toEqual([]);
  });

  it('flushes buffered keystrokes when the server confirms writer role', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onRole = vi.fn();
    const client = new TerminalClient({
      onData: () => {},
      onText: () => {},
      onRole,
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();

    const enc = new TextEncoder();
    client.sendInput(enc.encode('a'));
    client.sendInput(enc.encode('b'));
    client.sendInput(enc.encode('c'));

    FakeWebSocket.instances[0].openAsWriter();

    // The openAsWriter helper delivers the ready frame synchronously
    // after the open event. The flush happens inside handleText.
    const sent = FakeWebSocket.instances[0].sent;
    expect(sent).toHaveLength(3);
    expect(JSON.parse(sent[0])).toMatchObject({ type: 'input', data: 'YQ==' });
    expect(JSON.parse(sent[1])).toMatchObject({ type: 'input', data: 'Yg==' });
    expect(JSON.parse(sent[2])).toMatchObject({ type: 'input', data: 'Yw==' });
    expect(onRole).toHaveBeenCalledWith('writer');
  });

  it('does NOT flush keystrokes when the server assigns the reader role', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onRole = vi.fn();
    const client = new TerminalClient({
      onData: () => {},
      onText: () => {},
      onRole,
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();

    const enc = new TextEncoder();
    client.sendInput(enc.encode('x'));

    FakeWebSocket.instances[0].openAsReader();
    expect(FakeWebSocket.instances[0].sent).toEqual([]);
    expect(onRole).toHaveBeenCalledWith('reader');
  });

  it('preserves the typing order across buffering and flush', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const client = new TerminalClient({
      onData: () => {},
      onText: () => {},
      onRole: () => {},
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();

    const enc = new TextEncoder();
    const chars = ['l', 's', ' ', '-', 'l', 'a', '\r'];
    for (const c of chars) client.sendInput(enc.encode(c));

    FakeWebSocket.instances[0].openAsWriter();

    const sent = FakeWebSocket.instances[0].sent;
    expect(sent).toHaveLength(chars.length);
    for (let i = 0; i < chars.length; i++) {
      const frame = JSON.parse(sent[i]);
      expect(frame.type).toBe('input');
      const decoded = atob(frame.data);
      expect(decoded).toBe(chars[i]);
    }
  });

  it('drops the pending buffer on close so a stale session cannot leak', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const client = new TerminalClient({
      onData: () => {},
      onText: () => {},
      onRole: () => {},
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();

    const enc = new TextEncoder();
    client.sendInput(enc.encode('a'));
    client.close();
    // After close, opening a fresh socket and assigning writer must
    // not replay the stale buffer.
    FakeWebSocket.instances[0].openAsWriter();
    expect(FakeWebSocket.instances[0].sent).toEqual([]);
  });
});

describe('TerminalClient binary dedup', () => {
  // The PTY byte stream is monotonic. The client must accept a binary
  // frame whose first byte was not yet seen, and drop a frame whose
  // first byte is already covered. The new contract uses *byte* level
  // tracking (lastReceivedEnd) instead of chunk-level (lastReceivedSeq),
  // so a chunk that starts right after a previous chunk is still
  // accepted even though its seq equals endSeq of the previous dump.
  it('accepts the snapshot binary frame and updates the cursor to its last byte', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onData = vi.fn();
    const client = new TerminalClient({
      onData,
      onText: () => {},
      onRole: () => {},
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();
    FakeWebSocket.instances[0].openAsWriter();

    // A snapshot of 5 bytes starting at seq 0.
    const enc = new TextEncoder();
    const snapshot = enc.encode('hello');
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(0, snapshot) });

    expect(onData).toHaveBeenCalledTimes(1);
    expect(Array.from(onData.mock.calls[0][0])).toEqual(Array.from(snapshot));
    expect(client.lastSeq()).toBe(4); // last byte position
  });

  it('drops a chunk that overlaps the previous chunk (full duplicate)', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onData = vi.fn();
    const client = new TerminalClient({
      onData,
      onText: () => {},
      onRole: () => {},
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();
    FakeWebSocket.instances[0].openAsWriter();

    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(0, new TextEncoder().encode('hello')) });
    onData.mockClear();
    // Same chunk again.
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(0, new TextEncoder().encode('hello')) });
    expect(onData).not.toHaveBeenCalled();
  });

  it('accepts a chunk whose first byte is right after the last received byte', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onData = vi.fn();
    const client = new TerminalClient({
      onData,
      onText: () => {},
      onRole: () => {},
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();
    FakeWebSocket.instances[0].openAsWriter();

    // Snapshot of 5 bytes, seq 0..4.
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(0, new TextEncoder().encode('hello')) });
    onData.mockClear();
    // Next chunk starts at seq 5 — the byte position right after
    // the snapshot's last byte. This is exactly what the live
    // broadcast emits after the snapshot was taken (the readLoop
    // increments endSeq by len(snapshot), so the next chunk's
    // first-byte seq equals the snapshot's endSeq). With the old
    // chunk-level dedup seeded from the `ready` frame's end_seq
    // this was wrongly dropped.
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(5, new TextEncoder().encode('!')) });
    expect(onData).toHaveBeenCalledTimes(1);
  });
});

describe('TerminalClient end-to-end (prompt + first keypress)', () => {
  // Mirrors the user-facing UX bug:
  //   1. User opens the terminal.
  //   2. Server replays the rolling dump (which contains the prompt)
  //      as a binary frame, then a "live" chunk for the same prompt
  //      arrives via the pending flush right after.
  //   3. The user types 'l' immediately.
  // The prompt must appear, the prompt must not be duplicated, and
  // the keystroke must reach the server.
  it('renders the prompt without duplication and forwards the first keypress', async () => {
    const { TerminalClient } = await import('./terminalClient');
    const onData = vi.fn();
    const onRole = vi.fn();
    const client = new TerminalClient({
      onData,
      onText: () => {},
      onRole,
      onConnectionChange: () => {},
      onError: () => {},
    });
    client.start();

    // 1. User starts typing before the server has confirmed writer
    //    role. The keystroke must be queued, not dropped.
    const enc = new TextEncoder();
    client.sendInput(enc.encode('l'));

    // 2. Socket opens + ready (writer). Buffered keystroke is flushed.
    FakeWebSocket.instances[0].openAsWriter();
    expect(FakeWebSocket.instances[0].sent).toHaveLength(1);

    // 3. Server replays the rolling dump (the prompt bytes) as a
    //    binary frame.
    const prompt = enc.encode('root@df22b8c40be0:~# ');
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(0, prompt) });

    // 4. Server's pending flush delivers the same prompt bytes
    //    again, with seq starting at 0 (the readLoop emits with the
    //    byte-level seq, not a snapshot-watermark seq). The byte-
    //    level dedup must drop this duplicate.
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(0, prompt) });

    // 5. Server emits the echo of 'l' as a new chunk starting at
    //    seq = prompt.length.
    FakeWebSocket.instances[0].onmessage?.({ data: encodeBinary(prompt.length, enc.encode('l')) });

    // The prompt must have been delivered exactly once and the echo
    // of 'l' must have followed it.
    expect(onData).toHaveBeenCalledTimes(2);
    expect(Array.from(onData.mock.calls[0][0])).toEqual(Array.from(prompt));
    expect(Array.from(onData.mock.calls[1][0])).toEqual([0x6c]); // 'l'
    expect(onRole).toHaveBeenCalledWith('writer');
  });
});

function encodeBinary(seq: number, payload: Uint8Array): ArrayBuffer {
  const buf = new ArrayBuffer(8 + payload.length);
  const view = new DataView(buf);
  view.setBigUint64(0, BigInt(seq), false);
  new Uint8Array(buf, 8).set(payload);
  return buf;
}
