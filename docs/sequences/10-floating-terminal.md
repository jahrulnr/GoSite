# Sequence: Floating Terminal Session

What happens when the user clicks the topbar Terminal icon, types into the
xterm pane, and (optionally) refreshes or kills the session.

**Entry points:** topbar icon click → `terminalStore.toggleTerminal()` →
WebSocket upgrade at `/api/v1/terminal/ws?session_id=...`.

## Initial attach (no active session)

```mermaid
sequenceDiagram
    actor User
    participant Topbar
    participant Store as terminalStore
    participant API as terminalApi (REST)
    participant WS as terminal/ws
    participant Hub
    participant PTY as PtySession
    participant FS as /tmp/gosite-term-{sid}.log

    User->>Topbar: click Terminal icon
    Topbar->>Store: toggleTerminal()
    alt persisted activeSessionId
        Store->>API: GET /terminal/sessions
        API-->>Store: list of active sessions
        alt session found
            Store->>Store: keep activeSessionId
        else session swept or missing
            Store->>Store: clear activeSessionId
        end
    end
    Store->>WS: open WS (session_id=?)
    WS->>Hub: AttachOrCreate(ctx, userID, sessionId, opts)
    alt sessionId empty
        Hub->>PTY: spawn /bin/bash via ptyStart
    else sessionId present
        Hub->>FS: stat dumpPath
        alt dump present
            Hub->>PTY: spawn + restore firstSeq from size
        else
            Hub->>PTY: spawn new session, reuse id
        end
    end
    Hub->>WS: assign role (writer|reader)
    WS-->>User: ready frame (text)
    WS-->>User: snapshot binary [seq=N][dump]
    WS-->>User: live output (binary frames)
```

## Writer input → PTY → broadcast

```mermaid
sequenceDiagram
    actor User
    participant Xterm as TerminalPane
    participant Client as TerminalClient
    participant WS as terminal/ws
    participant Hub
    participant PTY as PtySession
    participant FS as dump file
    participant Readers as other attaches

    User->>Xterm: keystroke
    Xterm->>Client: sendInput(bytes)
    alt role == writer
        Client->>WS: {type:"input", data:base64}
        WS->>Hub: write to PTY master
        Hub->>PTY: master.Write
        PTY->>PTY: shell executes
        PTY-->>Hub: readLoop() chunk
        Hub->>FS: rolling.Append (trim 25% if > 256KB)
        Hub->>Hub: bump endSeq, dedup firstSeq
        Hub-->>WS: binary frame [seq][chunk]
        Hub-->>Readers: same binary frame
    else role == reader
        Client->>Client: drop (read-only)
    end
```

## Refresh / reconnect (no kill)

```mermaid
sequenceDiagram
    actor User
    participant Browser
    participant Store as terminalStore
    participant API as terminalApi
    participant WS as terminal/ws
    participant Hub
    participant FS as dump file

    User->>Browser: click Terminal icon after refresh
    Store->>API: GET /terminal/sessions
    API-->>Store: list
    Store->>Store: validate persisted activeSessionId
    Store->>WS: open WS ?session_id=...
    WS->>Hub: AttachOrCreate
    Hub-->>WS: ready frame {first_seq, end_seq, role}
    WS-->>Browser: snapshot binary [seq=first][dump]
    Browser->>Browser: dedup any chunk with seq <= lastReceivedSeq
    Note over Hub,FS: PTY is the same process; no spawn unless restart lost dump
```

## Server restart with `/tmp` mounted

```mermaid
sequenceDiagram
    participant Docker
    participant Hub
    participant FS as /tmp/gosite-term-{sid}.log
    participant PTY as new PtySession

    Docker->>Hub: container restart
    Note over Hub: in-memory registry cleared
    User->>Hub: open WS ?session_id=abc
    Hub->>FS: stat dumpPath
    FS-->>Hub: size=4096
    Hub->>PTY: spawn /bin/bash, FirstSeq=4096
    PTY-->>Hub: ready frame {first_seq:4096, end_seq:4096}
    Hub-->>User: snapshot binary [seq=4096][dump]
    Note over PTY: previous shell process is gone; this is a fresh shell<br/>but the scrollback survives via the rolling dump.
```

## Sweeper kill (12h idle)

```mermaid
sequenceDiagram
    participant Sweep as RunSweeper
    participant Hub
    participant PTY as PtySession
    participant FS as /tmp/gosite-term-{sid}.log

    loop every 1 minute
        Sweep->>Hub: walk sessions
        alt now - lastInput > 12h OR (no writer && now - lastAttach > 12h)
            Sweep->>Hub: Kill(id)
            Hub->>PTY: SIGKILL process group
            PTY->>FS: Remove()
            Hub->>Hub: unregister
        end
    end
```

## Multi-attach (1 writer + N readers)

```mermaid
sequenceDiagram
    actor Writer as Writer tab
    actor Reader as Reader tab
    participant Hub
    participant PTY

    Writer->>Hub: WS attach (no existing writer)
    Hub-->>Writer: ready {role: writer}
    Reader->>Hub: WS attach
    Hub-->>Reader: ready {role: reader}
    Writer->>Hub: keystroke forwarded
    Hub->>PTY: master.Write
    PTY-->>Hub: chunk
    Hub-->>Writer: binary frame
    Hub-->>Reader: same binary frame (no input)
    Note over Reader: xterm.options.disableStdin = true
```

## Configuration knobs

| Env var                 | Default  | Description                                  |
|-------------------------|----------|----------------------------------------------|
| `TERMINAL_STICKY_TTL`   | `12h`    | Time without input/attach before sweeper kill |
| `TERMINAL_DUMP_DIR`     | `/tmp`   | Rolling dump location (host-mount for restart survival) |
| `TERMINAL_DUMP_MAX`     | `262144` | Cap (bytes) before oldest 25% is dropped     |
| `TERMINAL_PER_USER_MAX` | `8`      | Maximum concurrent sessions per user          |
