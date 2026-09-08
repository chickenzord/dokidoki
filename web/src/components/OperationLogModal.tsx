import React, { useEffect, useRef, useState, useCallback, useImperativeHandle, forwardRef } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from './ui/dialog';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { Terminal as TerminalIcon, Copy, Check, Loader2, CheckCircle2, XCircle, AlertTriangle } from 'lucide-react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export interface TerminalViewHandle {
  getPlainText: () => string;
  getDimensions: () => { cols: number; rows: number };
}

interface TerminalViewProps {
  output: string;
  isRunning: boolean;
  onDimensions?: (dims: { cols: number; rows: number }) => void;
  onReady?: (dims: { cols: number; rows: number }) => void;
}

const TerminalView = forwardRef<TerminalViewHandle, TerminalViewProps>(
  ({ output, isRunning, onDimensions, onReady }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const termRef = useRef<XTerm | null>(null);
    const fitAddonRef = useRef<FitAddon | null>(null);
    const lastOutputLen = useRef(0);
    const isReadyReported = useRef(false);

    useImperativeHandle(ref, () => ({
      getPlainText: () => {
        const term = termRef.current;
        if (!term) return '';
        term.selectAll();
        const sel = term.getSelection();
        term.clearSelection();
        return sel;
      },
      getDimensions: () => {
        const term = termRef.current;
        return {
          cols: term?.cols || 80,
          rows: term?.rows || 24,
        };
      },
    }));

    useEffect(() => {
      if (!containerRef.current) return;

      const term = new XTerm({
        convertEol: false,
        disableStdin: true,
        cursorBlink: false,
        scrollback: 5000,
        fontFamily: 'monospace',
        fontSize: 12,
        lineHeight: 1.3,
        theme: {
          background: '#020617', // slate-950
          foreground: '#cbd5e1', // slate-300
          selectionBackground: '#334155', // slate-700
        },
      });

      const fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(containerRef.current);

      termRef.current = term;
      fitAddonRef.current = fitAddon;
      lastOutputLen.current = 0;
      isReadyReported.current = false;

      const safeFit = () => {
        try {
          if (containerRef.current && containerRef.current.clientWidth > 0 && containerRef.current.clientHeight > 0) {
            fitAddon.fit();
            if (term.cols && term.rows) {
              onDimensions?.({ cols: term.cols, rows: term.rows });
              if (!isReadyReported.current) {
                isReadyReported.current = true;
                onReady?.({ cols: term.cols, rows: term.rows });
              }
            }
          }
        } catch {
          // ignore layout errors during transitions
        }
      };

      safeFit();
      const timer1 = setTimeout(safeFit, 50);
      const timer2 = setTimeout(safeFit, 150);

      const ro = new ResizeObserver(() => {
        safeFit();
      });
      ro.observe(containerRef.current);

      return () => {
        clearTimeout(timer1);
        clearTimeout(timer2);
        ro.disconnect();
        term.dispose();
        termRef.current = null;
        fitAddonRef.current = null;
        lastOutputLen.current = 0;
        isReadyReported.current = false;
      };
    }, []);

    useEffect(() => {
      const term = termRef.current;
      if (!term) return;

      if (output.length < lastOutputLen.current) {
        term.reset();
        lastOutputLen.current = 0;
      }

      if (output.length > lastOutputLen.current) {
        const chunk = output.slice(lastOutputLen.current);
        term.write(chunk);
        lastOutputLen.current = output.length;
      }
    }, [output]);

    return (
      <div className="relative bg-slate-950 border border-slate-800 rounded-lg p-3 h-80 flex flex-col overflow-hidden">
        <div ref={containerRef} className="w-full flex-1 overflow-hidden" />
        {!output && (
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none text-slate-500 text-xs italic">
            {isRunning ? 'Waiting for output...' : 'No output received.'}
          </div>
        )}
      </div>
    );
  }
);
TerminalView.displayName = 'TerminalView';

const stripAnsi = (str: string): string => {
  return str
    .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')
    .replace(/\x1b\([a-zA-Z0-9]/g, '')
    .replace(/\r/g, '');
};

export interface OperationTask {
  title: string;
  subtitle?: string;
  action: (
    onChunk: (chunk: string) => void,
    dimensions?: { cols?: number; rows?: number }
  ) => Promise<unknown>;
  onSuccess?: () => void;
}

export interface OperationLogModalProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  subtitle?: string;
  output?: string;
  isRunning?: boolean;
  error?: string | null;
  success?: boolean;
  // When provided, the modal manages streaming internally to isolate frequent chunks from parent components
  operation?: OperationTask | null;
}

export const OperationLogModal: React.FC<OperationLogModalProps> = ({
  isOpen,
  onClose,
  title = '',
  subtitle,
  output = '',
  isRunning: controlledRunning = false,
  error: controlledError = null,
  success: controlledSuccess = false,
  operation,
}) => {
  const terminalRef = useRef<TerminalViewHandle>(null);
  const [copied, setCopied] = useState(false);
  const [internalOutput, setInternalOutput] = useState('');
  const [internalRunning, setInternalRunning] = useState(false);
  const [internalError, setInternalError] = useState<string | null>(null);
  const [internalSuccess, setInternalSuccess] = useState(false);
  const [readyDimensions, setReadyDimensions] = useState<{ cols: number; rows: number } | null>(null);
  const activeOperationRef = useRef<OperationTask | null>(null);
  const terminalDimsRef = useRef<{ cols: number; rows: number }>({ cols: 80, rows: 24 });

  const handleDimensions = useCallback((dims: { cols: number; rows: number }) => {
    terminalDimsRef.current = dims;
  }, []);

  const handleReady = useCallback((dims: { cols: number; rows: number }) => {
    terminalDimsRef.current = dims;
    setReadyDimensions((prev) => prev || dims);
  }, []);

  // Reset ready state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setReadyDimensions(null);
      activeOperationRef.current = null;
    }
  }, [isOpen]);

  // Fallback if measurement doesn't fire within 300ms (e.g. headless test or hidden tab)
  useEffect(() => {
    if (!isOpen || !operation || readyDimensions) return;
    const timer = setTimeout(() => {
      const liveDims = terminalRef.current?.getDimensions() || terminalDimsRef.current;
      if (liveDims && liveDims.cols > 0 && liveDims.rows > 0) {
        setReadyDimensions((prev) => prev || liveDims);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [isOpen, operation, readyDimensions]);

  // Trigger streaming action ONLY when terminal has been built and reported its actual dimensions
  useEffect(() => {
    if (!isOpen || !operation || !readyDimensions) return;
    if (activeOperationRef.current === operation) return;

    activeOperationRef.current = operation;
    setInternalOutput('');
    setInternalError(null);
    setInternalSuccess(false);
    setInternalRunning(true);

    operation
      .action((chunk) => {
        if (activeOperationRef.current === operation) {
          setInternalOutput((prev) => prev + chunk);
        }
      }, readyDimensions)
      .then(() => {
        if (activeOperationRef.current === operation) {
          setInternalSuccess(true);
          operation.onSuccess?.();
        }
      })
      .catch((err: any) => {
        if (activeOperationRef.current === operation) {
          setInternalError(err?.message || 'Operation failed');
        }
      })
      .finally(() => {
        if (activeOperationRef.current === operation) {
          setInternalRunning(false);
        }
      });
  }, [isOpen, operation, readyDimensions]);

  const activeTitle = operation?.title || title;
  const activeSubtitle = operation?.subtitle || subtitle;
  const activeOutput = operation ? internalOutput : output;
  const activeRunning = operation ? internalRunning : controlledRunning;
  const activeError = operation ? internalError : controlledError;
  const activeSuccess = operation ? internalSuccess : controlledSuccess;

  useEffect(() => {
    if (!activeRunning) return;

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [activeRunning]);

  const handleCopy = () => {
    const termText = terminalRef.current?.getPlainText();
    const baseText = termText && termText.trim().length > 0 ? termText : stripAnsi(activeOutput);
    const textToCopy = activeError ? `${baseText}\n\nError: ${activeError}` : baseText;
    navigator.clipboard.writeText(textToCopy);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && !activeRunning && onClose()}>
      <DialogContent className="w-full sm:max-w-2xl bg-slate-900 border-slate-800 text-slate-100">
        <DialogHeader>
          <div className="flex items-center justify-between pr-6">
            <DialogTitle className="text-lg font-bold text-white flex items-center gap-2">
              <TerminalIcon className="w-5 h-5 text-rose-500" />
              {activeTitle}
            </DialogTitle>
            {activeRunning ? (
              <Badge variant="outline" className="bg-amber-950/40 text-amber-400 border-amber-800/60 font-mono text-xs flex items-center gap-1.5">
                <Loader2 className="w-3 h-3 animate-spin" />
                Running
              </Badge>
            ) : activeError ? (
              <Badge variant="outline" className="bg-rose-950/40 text-rose-400 border-rose-800/60 font-mono text-xs flex items-center gap-1.5">
                <XCircle className="w-3 h-3" />
                Failed
              </Badge>
            ) : activeSuccess ? (
              <Badge variant="outline" className="bg-emerald-950/40 text-emerald-400 border-emerald-800/60 font-mono text-xs flex items-center gap-1.5">
                <CheckCircle2 className="w-3 h-3" />
                Success
              </Badge>
            ) : null}
          </div>
          {activeSubtitle && (
            <DialogDescription className="text-slate-400 text-xs">
              {activeSubtitle}
            </DialogDescription>
          )}
        </DialogHeader>

        <div className="space-y-3">
          {activeRunning && (
            <div className="p-2.5 bg-amber-950/30 border border-amber-800/50 rounded-lg text-amber-300 text-xs flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0" />
              <span>
                Please keep this page open. Navigating away or closing the tab may interrupt the running process.
              </span>
            </div>
          )}

          <div className="relative group">
            {isOpen && (
              <TerminalView
                ref={terminalRef}
                output={activeOutput}
                isRunning={activeRunning}
                onReady={handleReady}
                onDimensions={handleDimensions}
              />
            )}

            {activeOutput && (
              <Button
                size="icon"
                variant="outline"
                aria-label="Copy logs to clipboard"
                className="absolute top-2 right-2 h-7 w-7 bg-slate-900/80 border-slate-700 text-slate-300 hover:text-white hover:bg-slate-800 opacity-0 group-hover:opacity-100 transition-opacity z-10"
                onClick={handleCopy}
                title="Copy log"
              >
                {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              </Button>
            )}
          </div>

          {activeError && (
            <div className="p-3 bg-rose-950/30 border border-rose-800/50 rounded-lg text-rose-300 text-xs flex items-start gap-2">
              <XCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
              <div className="break-all font-mono">{activeError}</div>
            </div>
          )}
        </div>

        <DialogFooter className="flex items-center justify-between sm:justify-between pt-2">
          <div className="text-xs text-slate-500">
            {activeRunning ? 'Execution in progress, please wait...' : 'Operation finished.'}
          </div>
          <Button
            variant={activeRunning ? 'outline' : 'default'}
            size="sm"
            onClick={onClose}
            disabled={activeRunning}
            className={activeRunning ? 'border-slate-700 text-slate-400' : 'bg-rose-600 hover:bg-rose-700 text-white'}
          >
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
