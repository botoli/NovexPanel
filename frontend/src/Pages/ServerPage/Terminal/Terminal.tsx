import { useEffect, useRef } from 'react';
import { useParams } from 'react-router-dom';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { tokenStore } from '../../../Store/TokenStore';

export const TerminalPage = () => {
  const { id } = useParams();
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const sessionIdRef = useRef<string | null>(null);
  const pendingInputRef = useRef<string>('');

  useEffect(() => {
    if (!id) return;

    const serverId = Number.parseInt(id, 10);
    if (!Number.isFinite(serverId) || serverId <= 0) return;

    const isRecord = (v: unknown): v is Record<string, unknown> =>
      typeof v === 'object' && v !== null;

    // 1. Создать терминал
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      theme: {
        background: '#000000',
        foreground: '#f0f0f0',
        cursor: '#2DD4BF',
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    termRef.current = term;
    fitAddonRef.current = fitAddon;

    if (terminalRef.current) {
      term.open(terminalRef.current);
      fitAddon.fit();
      term.focus();
    }

    // 2. Открыть WebSocket
    const token = tokenStore.getToken();
    const ws = new WebSocket(`ws://localhost:8380/site/ws?token=${token}`);

    wsRef.current = ws;

    ws.onopen = () => {
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
      if (typeof event.data !== 'string') return;

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
      term.writeln(`\r\n\x1b[31mWebSocket error: ${JSON.stringify(error)}\x1b[0m`);
    };

    ws.onclose = () => {
      term.writeln('\r\n\x1b[33mConnection closed. Reload page to reconnect.\x1b[0m');
    };

    // 3. Отправка ввода
    term.onData((data) => {
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
      window.removeEventListener('resize', handleResize);
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'close_terminal', server_id: serverId }));
      }
      ws.close();
      term.dispose();
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
