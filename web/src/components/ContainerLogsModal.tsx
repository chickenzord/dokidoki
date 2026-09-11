import React, { useEffect, useRef, useState, useCallback } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from './ui/dialog';
import { Button } from './ui/button';
import { Input } from './ui/input';
import {
  Terminal as TerminalIcon,
  Maximize2,
  Minimize2,
  Download,
  RotateCcw,
  Pause,
  Play,
  X,
  WrapText,
  Clock,
  Search,
  Check,
  Copy,
} from 'lucide-react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { api } from '../services/api';

interface ContainerLogsModalProps {
  isOpen: boolean;
  onClose: () => void;
  containerId: string;
  containerName: string;
  nodeName: string;
  hostEndpoint?: string;
}

interface LogEntry {
  type: 'stdout' | 'stderr';
  line: string;
}

export const ContainerLogsModal: React.FC<ContainerLogsModalProps> = ({
  isOpen,
  onClose,
  containerId,
  containerName,
  nodeName,
  hostEndpoint,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);

  // Buffer of logs in memory for search, filtering, and export
  const logEntriesRef = useRef<LogEntry[]>([]);

  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [showTimestamps, setShowTimestamps] = useState(false);
  const [wrapLines, setWrapLines] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const searchTermRef = useRef(searchTerm);
  searchTermRef.current = searchTerm;
  const [copied, setCopied] = useState(false);

  const [isTerminalReady, setIsTerminalReady] = useState(false);

  // Write a single log entry to xterm
  const writeEntryToTerm = useCallback((term: XTerm, entry: LogEntry) => {
    if (entry.type === 'stderr') {
      term.writeln(`\x1b[33m${entry.line}\x1b[0m`);
    } else {
      term.writeln(entry.line);
    }
  }, []);

  // Re-render the terminal buffer when search term changes or when terminal becomes ready
  const refreshTerminalBuffer = useCallback(() => {
    const term = termRef.current;
    if (!term) return;

    term.clear();
    const filter = searchTermRef.current.trim().toLowerCase();
    const entries = logEntriesRef.current;

    for (const entry of entries) {
      if (!filter || entry.line.toLowerCase().includes(filter)) {
        writeEntryToTerm(term, entry);
      }
    }
  }, [writeEntryToTerm]);

  // Connect to SSE stream
  const connectSSE = useCallback(() => {
    if (!containerId) return;

    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    const url = api.getContainerLogsUrl(
      containerId,
      {
        follow: true,
        tail: '500',
        timestamps: showTimestamps,
      },
      hostEndpoint
    );

    const es = new EventSource(url);
    eventSourceRef.current = es;

    const handleMessage = (type: 'stdout' | 'stderr', data: string) => {
      const entry: LogEntry = { type, line: data };
      logEntriesRef.current.push(entry);

      const filter = searchTermRef.current.trim().toLowerCase();
      const term = termRef.current;
      if (term && (!filter || data.toLowerCase().includes(filter))) {
        writeEntryToTerm(term, entry);
      }
    };

    es.addEventListener('stdout', (e) => {
      handleMessage('stdout', e.data);
    });

    es.addEventListener('stderr', (e) => {
      handleMessage('stderr', e.data);
    });

    es.onerror = (err) => {
      console.warn('EventSource failed for container logs:', err);
    };
  }, [containerId, hostEndpoint, showTimestamps, writeEntryToTerm]);

  // Terminal lifecycle & fit handler
  useEffect(() => {
    if (!isOpen) return;

    // Defer to next frame: Radix Dialog's Portal may mount the content after
    // this effect's commit, leaving containerRef.current null on first run.
    const rafId = requestAnimationFrame(() => {
      if (!containerRef.current) return;

      setIsTerminalReady(false);

      const term = new XTerm({
        convertEol: true,
        disableStdin: true,
        fontSize: 12,
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
        theme: {
          background: '#020617',
          foreground: '#e2e8f0',
          cursor: 'transparent',
        },
        scrollback: 5000,
      });

      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(containerRef.current);

      termRef.current = term;
      fitAddonRef.current = fitAddon;
      setIsTerminalReady(true);

      const safeFit = () => {
        try {
          if (containerRef.current && containerRef.current.clientWidth > 0 && containerRef.current.clientHeight > 0) {
            fitAddon.fit();
          }
        } catch {
          // ignore layout fit transitions
        }
      };

      safeFit();
      const timer1 = setTimeout(safeFit, 50);
      const timer2 = setTimeout(safeFit, 150);

      const ro = new ResizeObserver(() => safeFit());
      ro.observe(containerRef.current);

      cleanupRef.current = () => {
        clearTimeout(timer1);
        clearTimeout(timer2);
        ro.disconnect();
        setIsTerminalReady(false);
        term.dispose();
        termRef.current = null;
        fitAddonRef.current = null;
      };
    });

    return () => {
      cancelAnimationFrame(rafId);
      if (cleanupRef.current) {
        cleanupRef.current();
        cleanupRef.current = null;
      }
    };
  }, [isOpen]);

  // Stream connection lifecycle: governed by isOpen, containerId, hostEndpoint, timestamps, and isPaused
  useEffect(() => {
    if (!isOpen || !containerId || isPaused) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      return;
    }

    // Reset local log entries on new connection or timestamp switch
    logEntriesRef.current = [];
    termRef.current?.clear();

    connectSSE();

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [isOpen, containerId, hostEndpoint, showTimestamps, isPaused, connectSSE]);

  // Flush buffered logs when terminal becomes ready
  useEffect(() => {
    if (isTerminalReady && logEntriesRef.current.length > 0) {
      refreshTerminalBuffer();
    }
  }, [isTerminalReady, refreshTerminalBuffer]);

  // Re-fit when fullscreen changes
  useEffect(() => {
    const timer = setTimeout(() => {
      try {
        fitAddonRef.current?.fit();
      } catch {
        // ignore
      }
    }, 100);
    return () => clearTimeout(timer);
  }, [isFullscreen]);

  // Re-filter when search term changes
  useEffect(() => {
    refreshTerminalBuffer();
  }, [searchTerm, refreshTerminalBuffer]);

  const handleClear = () => {
    logEntriesRef.current = [];
    termRef.current?.clear();
  };

  const handleCopy = () => {
    const term = termRef.current;
    if (!term) return;
    term.selectAll();
    const sel = term.getSelection();
    term.clearSelection();
    if (sel) {
      navigator.clipboard.writeText(sel);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleDownload = () => {
    const cleanNode = (nodeName || 'local').replace(/[^a-zA-Z0-9._-]/g, '_');
    const cleanContainer = (containerName || 'container').replace(/^\//, '').replace(/[^a-zA-Z0-9._-]/g, '_');
    const now = new Date();
    const pad = (n: number) => String(n).padStart(2, '0');
    const ts = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}_${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
    const filename = `${cleanNode}_${cleanContainer}_${ts}.log`;

    const text = logEntriesRef.current.map((e) => e.line).join('\n');
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const cleanName = containerName.replace(/^\//, '');

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        hideDefaultClose
        className={`${
          isFullscreen
            ? 'w-[98vw] max-w-[98vw] h-[96vh] max-h-[96vh]'
            : 'w-full max-w-4xl h-[650px] max-h-[90vh]'
        } !flex !flex-col p-0 gap-0 bg-slate-900 border-slate-800 text-slate-100 overflow-hidden transition-all duration-150`}
      >
        {/* Header */}
        <DialogHeader className="px-4 py-3 border-b border-slate-800 flex flex-row items-center justify-between space-y-0 flex-shrink-0">
          <div className="flex items-center gap-2 min-w-0 pr-4">
            <TerminalIcon className="w-5 h-5 text-indigo-400 flex-shrink-0" />
            <div className="flex flex-col min-w-0">
              <DialogTitle className="text-sm font-semibold truncate flex items-center gap-2 text-white">
                <span className="truncate">{cleanName}</span>
                <span className="text-xs font-mono font-normal text-slate-400">({nodeName || 'local'})</span>
              </DialogTitle>
              <DialogDescription className="text-[11px] text-slate-400">
                Container logs stream
              </DialogDescription>
            </div>
          </div>

          {/* Controls toolbar */}
          <div className="flex items-center gap-1.5 flex-shrink-0">
            {/* Search filter input */}
            <div className="relative">
              <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none" />
              <Input
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Filter logs…"
                className="h-7 pl-7 pr-2 text-xs bg-slate-950 border-slate-700 w-32 sm:w-44 text-slate-200 placeholder:text-slate-500 focus-visible:ring-1 focus-visible:ring-indigo-500"
              />
            </div>

            {/* Live Tail Pause/Resume */}
            <Button
              size="sm"
              variant="outline"
              onClick={() => setIsPaused(!isPaused)}
              title={isPaused ? 'Resume live tail' : 'Pause live tail'}
              className={`h-7 px-2 text-xs border-slate-700 ${
                isPaused
                  ? 'bg-amber-950/40 text-amber-400 border-amber-800/60 hover:bg-amber-900/50'
                  : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
              }`}
            >
              {isPaused ? <Play className="w-3.5 h-3.5 mr-1" /> : <Pause className="w-3.5 h-3.5 mr-1" />}
              <span>{isPaused ? 'Paused' : 'Live'}</span>
            </Button>

            {/* Timestamps toggle */}
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowTimestamps(!showTimestamps)}
              title={showTimestamps ? 'Hide timestamps' : 'Show timestamps'}
              className={`h-7 px-2 text-xs border-slate-700 ${
                showTimestamps
                  ? 'bg-indigo-950/40 text-indigo-400 border-indigo-800/60'
                  : 'bg-slate-800 text-slate-400 hover:bg-slate-700'
              }`}
            >
              <Clock className="w-3.5 h-3.5" />
            </Button>

            {/* Wrap toggle */}
            <Button
              size="sm"
              variant="outline"
              onClick={() => setWrapLines(!wrapLines)}
              title={wrapLines ? 'Disable line wrap' : 'Enable line wrap'}
              className={`h-7 px-2 text-xs border-slate-700 ${
                wrapLines
                  ? 'bg-indigo-950/40 text-indigo-400 border-indigo-800/60'
                  : 'bg-slate-800 text-slate-400 hover:bg-slate-700'
              }`}
            >
              <WrapText className="w-3.5 h-3.5" />
            </Button>

            {/* Clear Screen */}
            <Button
              size="sm"
              variant="outline"
              onClick={handleClear}
              title="Clear terminal buffer"
              className="h-7 px-2 text-xs bg-slate-800 text-slate-300 border-slate-700 hover:bg-slate-700"
            >
              <RotateCcw className="w-3.5 h-3.5" />
            </Button>

            {/* Copy */}
            <Button
              size="sm"
              variant="outline"
              onClick={handleCopy}
              title="Copy terminal selection or all"
              className="h-7 px-2 text-xs bg-slate-800 text-slate-300 border-slate-700 hover:bg-slate-700"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </Button>

            {/* Download */}
            <Button
              size="sm"
              variant="outline"
              onClick={handleDownload}
              title="Download logs"
              className="h-7 px-2 text-xs bg-slate-800 text-slate-300 border-slate-700 hover:bg-slate-700"
            >
              <Download className="w-3.5 h-3.5" />
            </Button>

            {/* Maximize / Minimize */}
            <Button
              size="sm"
              variant="outline"
              onClick={() => setIsFullscreen(!isFullscreen)}
              title={isFullscreen ? 'Exit fullscreen' : 'Maximize'}
              className="h-7 px-2 text-xs bg-slate-800 text-slate-300 border-slate-700 hover:bg-slate-700"
            >
              {isFullscreen ? <Minimize2 className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
            </Button>

            {/* Close */}
            <Button
              size="sm"
              variant="ghost"
              onClick={onClose}
              title="Close"
              className="h-7 w-7 p-0 text-slate-400 hover:text-white hover:bg-slate-800"
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
        </DialogHeader>

        {/* Terminal container */}
        <div
          className={`flex-1 min-h-0 bg-slate-950 p-2 overflow-hidden flex flex-col ${
            wrapLines ? 'whitespace-pre-wrap break-all' : ''
          }`}
        >
          <div ref={containerRef} className="flex-1 min-h-0 overflow-hidden" />
        </div>
      </DialogContent>
    </Dialog>
  );
};

