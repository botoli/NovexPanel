import { useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { tokenStore } from '../../../Store/TokenStore';
import { settingsStore } from '../../../Store/SettingsStore';
const WS_BASE = import.meta.env.VITE_WS_URL || 'ws://localhost:8380';

export const TerminalPage = () => {
  const { id } = useParams();
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const sessionIdRef = useRef<string | null>(null);
  const pendingInputRef = useRef<string>('');
  const onDataDisposableRef = useRef<{ dispose: () => void } | null>(null);
  const lastInputRef = useRef<{ data: string; at: number; }>({ data: '', at: 0 });

  useEffect(() => {
    if (!id) return;

    const serverId = Number.parseInt(id, 10);
    if (!Number.isFinite(serverId) || serverId <= 0) return;
    let disposed = false;

    const isRecord = (v: unknown): v is Record<string, unknown> =>
      typeof v === 'object' && v !== null;

    // Important for React StrictMode/dev: ensure we never have 2 active terminals/sockets.
    onDataDisposableRef.current?.dispose();
    onDataDisposableRef.current = null;
    try {
      wsRef.current?.close();
    } catch {
      // ignore
    }
    wsRef.current = null;
    try {
      termRef.current?.dispose();
    } catch {
      // ignore
    }
    termRef.current = null;
    fitAddonRef.current = null;

    // 1. Создать терминал
    const themeMode = settingsStore.state.terminalTheme;
    const terminalTheme = themeMode === 'classic'
      ? { background: '#000000', foreground: '#ffffff', cursor: '#ffffff' }
      : { background: '#000000', foreground: '#f0f0f0', cursor: '#2DD4BF' };
    const term = new Terminal({
      cursorBlink: true,
      fontSize: settingsStore.state.terminalFontSize || 14,
      cursorStyle: settingsStore.state.terminalCursorStyle,
      scrollback: settingsStore.state.terminalScrollback || 5000,
      disableStdin: false,
      theme: {
        ...terminalTheme,
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    termRef.current = term;
    fitAddonRef.current = fitAddon;

    if (terminalRef.current) {
      // Ensure no previous xterm instance is attached.
      terminalRef.current.innerHTML = '';
      term.open(terminalRef.current);
      fitAddon.fit();
      term.focus();
    }

    // 2. Открыть WebSocket
    const token = tokenStore.getToken();
    const ws = new WebSocket(`${WS_BASE}/site/ws?token=${token}`);

    wsRef.current = ws;

    ws.onopen = () => {
      if (disposed) return;
      term.writeln('Connected. Opening agent terminal...\r\n');
      sessionIdRef.current = null;
      pendingInputRef.current = '';
      ws.send(
        JSON.stringify({
          type: 'open_terminal',
          server_id: serverId,
          rows: term.rows,
          cols: term.cols,
        }),
      );
    };

    ws.onmessage = (event) => {
      if (disposed) return;
      if (wsRef.current !== ws) return;
      if (typeof event.data !== 'string') {
        // Some environments deliver WS text as Blob/ArrayBuffer.
        if (event.data instanceof Blob) {
          event.data
            .text()
            .then((text) => {
              if (disposed) return;
              if (wsRef.current !== ws) return;
              ws.onmessage?.({ ...event, data: text } as MessageEvent);
            })
            .catch(() => {});
        } else if (event.data instanceof ArrayBuffer) {
          try {
            const text = new TextDecoder().decode(event.data);
            if (disposed) return;
            if (wsRef.current !== ws) return;
            ws.onmessage?.({ ...event, data: text } as MessageEvent);
          } catch {
            // ignore
          }
        }
        return;
      }

      let msg: unknown;
      try {
        msg = JSON.parse(event.data);
      } catch {
        // Если по ошибке прилетела строка без JSON — просто печатаем.
        term.write(event.data);
        return;
      }

      if (!isRecord(msg)) return;

      const type = msg.type;
      if (typeof type !== 'string') return;

      switch (type) {
        case 'connected':
          return;
        case 'terminal_opened': {
          if (msg.server_id !== undefined && typeof msg.server_id === 'number' && msg.server_id !== serverId) {
            return;
          }
          const sessionId = typeof msg.session_id === 'string' ? msg.session_id : null;
          if (!sessionId) return;
          sessionIdRef.current = sessionId;

          const pending = pendingInputRef.current;
          if (pending) {
            pendingInputRef.current = '';
            ws.send(
              JSON.stringify({
                type: 'terminal_input',
                server_id: serverId,
                session_id: sessionId,
                data: pending,
              }),
            );
          }
          return;
        }
        case 'terminal_output': {
          const activeSessionId = sessionIdRef.current;
          const msgSessionId = typeof msg.session_id === 'string' ? msg.session_id : null;
          if (activeSessionId && msgSessionId && msgSessionId !== activeSessionId) {
            return;
          }
          if (typeof msg.server_id === 'number' && msg.server_id !== serverId) {
            return;
          }
          if (typeof msg.data === 'string') {
            term.write(msg.data);
          }
          return;
        }
        case 'error': {
          const errText = typeof msg.error === 'string' ? msg.error : 'Unknown error';
          term.writeln(`\r\n\x1b[31m${errText}\x1b[0m`);
          return;
        }
        default:
          return;
      }
    };

    ws.onerror = (error) => {
      if (disposed) return;
      term.writeln(`\r\n\x1b[31mWebSocket error: ${JSON.stringify(error)}\x1b[0m`);
    };

    ws.onclose = () => {
      if (disposed) return;
      term.writeln('\r\n\x1b[33mConnection closed. Reload page to reconnect.\x1b[0m');
    };

    // 3. Отправка ввода
    onDataDisposableRef.current = term.onData((data) => {
      if (disposed) return;
      if (wsRef.current !== ws) return;

      // Guard against duplicated onData events (common in some environments).
      // We only dedupe single-character inputs within a very small time window
      // to avoid breaking paste/multi-char inputs.
      const now = performance.now();
      if (data.length === 1) {
        const last = lastInputRef.current;
        if (last.data === data && now-last.at < 8) {
          return;
        }
        lastInputRef.current = { data, at: now };
      } else {
        lastInputRef.current = { data: '', at: now };
      }

      if (ws.readyState === WebSocket.OPEN) {
        const sessionId = sessionIdRef.current;
        if (!sessionId) {
          pendingInputRef.current += data;
          return;
        }
        ws.send(
          JSON.stringify({
            type: 'terminal_input',
            server_id: serverId,
            session_id: sessionId,
            data,
          }),
        );
      }
    });

    // 4. Ресайз при изменении окна
    const handleResize = () => {
      if (fitAddon) fitAddon.fit();
      if (ws.readyState === WebSocket.OPEN && term) {
        const sessionId = sessionIdRef.current;
        if (!sessionId) return;
        ws.send(
          JSON.stringify({
            type: 'terminal_resize',
            server_id: serverId,
            session_id: sessionId,
            cols: term.cols,
            rows: term.rows,
          }),
        );
      }
    };
    window.addEventListener('resize', handleResize);

    // 5. Очистка
    return () => {
      disposed = true;
      window.removeEventListener('resize', handleResize);
      onDataDisposableRef.current?.dispose();
      onDataDisposableRef.current = null;
      ws.onopen = null;
      ws.onmessage = null;
      ws.onerror = null;
      ws.onclose = null;
      try {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'close_terminal', server_id: serverId }));
        }
      } catch {
        // ignore
      }
      try {
        ws.close(1000, 'cleanup');
      } catch {
        // ignore
      }
      try {
        term.dispose();
      } catch {
        // ignore
      }
    };
  }, [id]);

  return (
    <div
      ref={terminalRef}
      style={{
        width: '100%',
        height: '100%',
        minHeight: '500px',
        overflow: 'hidden',

        borderRadius: '4px',
      }}
    />
  );
};
