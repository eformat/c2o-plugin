import * as React from 'react';
import { Terminal as XTerminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import 'xterm/css/xterm.css';

type TerminalProps = {
  onData: (data: string) => void;
  onResize: (cols: number, rows: number) => void;
};

export type ImperativeTerminalType = {
  focus: () => void;
  reset: () => void;
  onDataReceived: (data: string) => void;
  onConnectionClosed: (msg: string) => void;
};

const Terminal = React.forwardRef<ImperativeTerminalType, TerminalProps>(
  ({ onData, onResize }, ref) => {
    const terminal = React.useRef<XTerminal>();
    const containerRef = React.useRef<HTMLDivElement>();

    React.useEffect(() => {
      const term = new XTerminal({
        fontFamily: 'monospace',
        fontSize: 14,
        cursorBlink: true,
        cols: 80,
        rows: 25,
      });
      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(containerRef.current);
      try { fitAddon.fit(); } catch (_) { /* container may have zero dimensions */ }
      term.focus();

      const resizeObserver = new ResizeObserver(() => {
        window.requestAnimationFrame(() => {
          try { fitAddon.fit(); } catch (_) { /* ignore resize when unmounted */ }
        });
      });
      resizeObserver.observe(containerRef.current);

      if (terminal.current !== term) {
        terminal.current && terminal.current.dispose();
        terminal.current = term;
      }

      return () => {
        term.dispose();
        resizeObserver.disconnect();
      };
    }, []);

    React.useEffect(() => {
      const term = terminal.current;
      if (!term) return;
      const dataDisposable = term.onData(onData);
      const resizeDisposable = term.onResize(({ cols, rows }) => onResize(cols, rows));
      return () => {
        dataDisposable.dispose();
        resizeDisposable.dispose();
      };
    }, [onData, onResize]);

    React.useImperativeHandle(ref, () => ({
      focus: () => {
        terminal.current && terminal.current.focus();
      },
      reset: () => {
        if (!terminal.current) return;
        terminal.current.reset();
        terminal.current.clear();
        terminal.current.options.disableStdin = false;
      },
      onDataReceived: (data: string) => {
        terminal.current && terminal.current.write(data);
      },
      onConnectionClosed: (msg: string) => {
        if (!terminal.current) return;
        terminal.current.write(`\x1b[31m${msg || 'disconnected'}\x1b[m\r\n`);
        terminal.current.options.disableStdin = true;
      },
    }));

    return <div className="c2o-terminal" ref={containerRef} />;
  },
);

export default Terminal;
