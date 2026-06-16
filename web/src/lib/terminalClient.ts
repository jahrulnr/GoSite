// WebSocket client for the floating terminal.
//
// Protocol recap (matches internal/terminal/protocol.go):
//   - server -> client text : { type: 'ready' | 'exit' | 'error' | 'pong', ... }
//   - server -> client binary: [8 byte BE seq][raw PTY bytes]
//   - client -> server text : { type: 'input' | 'resize' | 'ping' | 'replay' }
//
// The client dedupes binary frames using the monotonic seq so that reconnects
// can replay missed output without double-printing bytes the server already
// delivered.
//
// Role is set by the server in the `ready` frame: 'writer' enables local
// keystroke forwarding; 'reader' is read-only.
import { API_BASE } from '../api/client';

export type Role = 'writer' | 'reader' | 'none';
export type ConnectionState = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'closed' | 'error';

export interface TerminalClientOptions {
  sessionId?: string;
  cols?: number;
  rows?: number;
  onData: (chunk: Uint8Array) => void;
  onText: (frame: TerminalTextFrame) => void;
  onRole: (role: Role) => void;
  onConnectionChange: (state: ConnectionState) => void;
  onError: (err: string) => void;
}

export type TerminalTextFrame =
  | { type: 'ready'; session_id: string; shell: string; cwd: string; cols: number; rows: number; role: Role; buffered_bytes: number; first_seq: number; end_seq: number; sticky_ttl: string; started_at: string }
  | { type: 'exit'; code: number }
  | { type: 'error'; message: string }
  | { type: 'pong' };

const RECONNECT_MIN_MS = 1000;
const RECONNECT_MAX_MS = 30000;

export class TerminalClient {
  private ws: WebSocket | null = null;
  private closed = false;
  private attempt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  // Position (seq) of the last PTY byte the client has actually written
  // to xterm. Initialized to -1 so the first chunk is always accepted.
  // We deliberately do NOT seed this from the `ready` frame's end_seq:
  // the snapshot binary frame that the server sends next carries the
  // authoritative range, and the pending flush can carry bytes whose
  // seq is < end_seq. Tracking the last *byte* (not the next seq)
  // makes the dedup correct for both the initial snapshot and live
  // chunks, while still dropping true duplicates on reconnect.
  private lastReceivedEnd = -1;
  private currentRole: Role = 'none';
  private currentSession: TerminalClientOptions;
  // Keystrokes that arrive before the socket is open *and* the server has
  // confirmed the writer role are buffered here. They are flushed in order
  // once both conditions are met. Without this queue the very first
  // character typed on a freshly opened terminal is silently dropped
  // (return false from sendInput), which is the source of the "1
  // character lost" UX bug.
  private pendingInput: Uint8Array[] = [];

  constructor(opts: TerminalClientOptions) {
    this.currentSession = opts;
  }

  start() {
    this.closed = false;
    this.connect();
  }

  close() {
    this.closed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.pendingInput = [];
    this.currentSession.onConnectionChange('closed');
  }

  sendInput(bytes: Uint8Array) {
    if (this.currentRole !== 'writer') {
      this.pendingInput.push(bytes);
      return true;
    }
    if (this.ws?.readyState !== WebSocket.OPEN) {
      this.pendingInput.push(bytes);
      return true;
    }
    this.flushInput(bytes);
    return true;
  }

  private flushInput(bytes: Uint8Array) {
    const frame = { type: 'input', data: bytesToBase64(bytes) };
    this.ws?.send(JSON.stringify(frame));
  }

  private flushPendingInput() {
    if (this.pendingInput.length === 0) return;
    if (this.currentRole !== 'writer') return;
    if (this.ws?.readyState !== WebSocket.OPEN) return;
    const queued = this.pendingInput;
    this.pendingInput = [];
    for (const chunk of queued) {
      this.flushInput(chunk);
    }
  }

  sendResize(cols: number, rows: number) {
    if (this.ws?.readyState !== WebSocket.OPEN) return;
    const frame = { type: 'resize', cols, rows };
    this.ws.send(JSON.stringify(frame));
  }

  sendPing() {
    if (this.ws?.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify({ type: 'ping' }));
  }

  // Replay requests a snapshot from sinceSeq onward. The server replies with
  // a single binary frame tagged with firstSeq=sinceSeq so the client can
  // dedupe if it already has those bytes.
  sendReplay(sinceSeq: number) {
    if (this.ws?.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify({ type: 'replay', since_seq: sinceSeq }));
  }

  role(): Role {
    return this.currentRole;
  }

  lastSeq(): number {
    return this.lastReceivedEnd;
  }

  private connect() {
    if (this.closed) return;
    this.currentSession.onConnectionChange(this.attempt === 0 ? 'connecting' : 'reconnecting');
    const url = buildWsUrl(this.currentSession.sessionId, this.currentSession.cols ?? 80, this.currentSession.rows ?? 24);
    let socket: WebSocket;
    try {
      socket = new WebSocket(url);
    } catch (err) {
      this.scheduleReconnect(err instanceof Error ? err.message : 'WebSocket ctor failed');
      return;
    }
    socket.binaryType = 'arraybuffer';
    this.ws = socket;
    socket.onopen = () => {
      this.attempt = 0;
      this.currentSession.onConnectionChange('connected');
      // The socket is now writable. If keystrokes were buffered before
      // the connect completed (and the role is already 'writer' from a
      // previous session's `ready` frame in a re-attach) flush them.
      this.flushPendingInput();
    };
    socket.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        this.handleText(ev.data);
      } else {
        this.handleBinary(ev.data as ArrayBuffer);
      }
    };
    socket.onerror = () => {
      this.currentSession.onError('websocket error');
    };
    socket.onclose = () => {
      this.ws = null;
      if (!this.closed) {
        this.scheduleReconnect('socket closed');
      }
    };
  }

  private handleText(raw: string) {
    let parsed: TerminalTextFrame;
    try {
      parsed = JSON.parse(raw) as TerminalTextFrame;
    } catch {
      this.currentSession.onError('malformed text frame');
      return;
    }
    if (parsed.type === 'ready') {
      this.currentRole = parsed.role;
      this.currentSession.sessionId = parsed.session_id;
      this.currentSession.onRole(parsed.role);
      // The ready frame is the first authoritative signal that the
      // server has accepted us as a writer. Flush any keystrokes that
      // arrived in the meantime.
      this.flushPendingInput();
      // We intentionally do NOT seed `lastReceivedEnd` from
      // `parsed.end_seq` here — see the field docstring.
    }
    this.currentSession.onText(parsed);
  }

  private handleBinary(buf: ArrayBuffer) {
    if (buf.byteLength < 8) return;
    const view = new DataView(buf);
    const seq = Number(view.getBigUint64(0, false));
    const payload = new Uint8Array(buf, 8);
    // Byte-level dedup: drop the chunk iff the first byte of the
    // payload was already covered by a previously received chunk.
    // This makes the initial snapshot binary frame (seq = firstSeq)
    // and the subsequent pending-flush chunks (which can have seq
    // values that overlap with the snapshot) cooperate correctly.
    if (seq <= this.lastReceivedEnd) {
      return;
    }
    this.lastReceivedEnd = seq + payload.length - 1;
    this.currentSession.onData(payload);
  }

  private scheduleReconnect(reason: string) {
    if (this.closed) return;
    const backoff = Math.min(RECONNECT_MAX_MS, RECONNECT_MIN_MS * 2 ** Math.min(this.attempt, 5));
    this.attempt += 1;
    this.currentSession.onConnectionChange('reconnecting');
    this.currentSession.onError(reason);
    this.reconnectTimer = setTimeout(() => this.connect(), backoff);
  }
}

function buildWsUrl(sessionId: string | undefined, cols: number, rows: number): string {
  const base = API_BASE.replace(/^http/, 'ws');
  const params = new URLSearchParams();
  if (sessionId) params.set('session_id', sessionId);
  params.set('cols', String(cols));
  params.set('rows', String(rows));
  return `${base}/terminal/ws?${params.toString()}`;
}

function bytesToBase64(bytes: Uint8Array): string {
  if (typeof btoa === 'function') {
    let bin = '';
    for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    return btoa(bin);
  }
  // Node fallback (used by vitest).
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const Buffer = (globalThis as any).Buffer;
  if (Buffer) return Buffer.from(bytes).toString('base64');
  // Last resort: hand-rolled encoder.
  return manualBtoa(bytes);
}

function manualBtoa(bytes: Uint8Array): string {
  const tbl = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  let out = '';
  for (let i = 0; i < bytes.length; i += 3) {
    const a = bytes[i];
    const b = i + 1 < bytes.length ? bytes[i + 1] : 0;
    const c = i + 2 < bytes.length ? bytes[i + 2] : 0;
    const triplet = (a << 16) | (b << 8) | c;
    out += tbl[(triplet >> 18) & 0x3f];
    out += tbl[(triplet >> 12) & 0x3f];
    out += i + 1 < bytes.length ? tbl[(triplet >> 6) & 0x3f] : '=';
    out += i + 2 < bytes.length ? tbl[triplet & 0x3f] : '=';
  }
  return out;
}
