import React, { useEffect, useRef, useState } from 'react';
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
import { Terminal, Copy, Check, Loader2, CheckCircle2, XCircle, AlertTriangle } from 'lucide-react';

interface OperationLogModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  output: string;
  isRunning: boolean;
  error?: string | null;
  success?: boolean;
}

export const OperationLogModal: React.FC<OperationLogModalProps> = ({
  isOpen,
  onClose,
  title,
  subtitle,
  output,
  isRunning,
  error,
  success,
}) => {
  const [copied, setCopied] = useState(false);
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isRunning) return;

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, [isRunning]);

  useEffect(() => {
    if (logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [output, isRunning]);

  const handleCopy = () => {
    const textToCopy = error ? `${output}\n\nError: ${error}` : output;
    navigator.clipboard.writeText(textToCopy);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && !isRunning && onClose()}>
      <DialogContent className="w-full sm:max-w-2xl bg-slate-900 border-slate-800 text-slate-100">
        <DialogHeader>
          <div className="flex items-center justify-between pr-6">
            <DialogTitle className="text-lg font-bold text-white flex items-center gap-2">
              <Terminal className="w-5 h-5 text-rose-500" />
              {title}
            </DialogTitle>
            {isRunning ? (
              <Badge variant="outline" className="bg-amber-950/40 text-amber-400 border-amber-800/60 font-mono text-xs flex items-center gap-1.5">
                <Loader2 className="w-3 h-3 animate-spin" />
                Running
              </Badge>
            ) : error ? (
              <Badge variant="outline" className="bg-rose-950/40 text-rose-400 border-rose-800/60 font-mono text-xs flex items-center gap-1.5">
                <XCircle className="w-3 h-3" />
                Failed
              </Badge>
            ) : success ? (
              <Badge variant="outline" className="bg-emerald-950/40 text-emerald-400 border-emerald-800/60 font-mono text-xs flex items-center gap-1.5">
                <CheckCircle2 className="w-3 h-3" />
                Success
              </Badge>
            ) : null}
          </div>
          {subtitle && (
            <DialogDescription className="text-slate-400 text-xs">
              {subtitle}
            </DialogDescription>
          )}
        </DialogHeader>

        <div className="space-y-3">
          {isRunning && (
            <div className="p-2.5 bg-amber-950/30 border border-amber-800/50 rounded-lg text-amber-300 text-xs flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0" />
              <span>
                Please keep this page open. Navigating away or closing the tab may interrupt the running process.
              </span>
            </div>
          )}

          <div className="relative group">
            <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-300 max-h-80 min-h-[140px] overflow-y-auto whitespace-pre-wrap break-all select-text leading-relaxed">
              {output ? (
                output
              ) : isRunning ? (
                <span className="text-slate-500 italic">Waiting for output...</span>
              ) : (
                <span className="text-slate-500 italic">No output received.</span>
              )}
              <div ref={logEndRef} />
            </div>

            {output && (
              <Button
                size="icon"
                variant="outline"
                className="absolute top-2 right-2 h-7 w-7 bg-slate-900/80 border-slate-700 text-slate-300 hover:text-white hover:bg-slate-800 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={handleCopy}
                title="Copy log"
              >
                {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              </Button>
            )}
          </div>

          {error && (
            <div className="p-3 bg-rose-950/30 border border-rose-800/50 rounded-lg text-rose-300 text-xs flex items-start gap-2">
              <XCircle className="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
              <div className="break-all font-mono">{error}</div>
            </div>
          )}
        </div>

        <DialogFooter className="flex items-center justify-between sm:justify-between pt-2">
          <div className="text-xs text-slate-500">
            {isRunning ? 'Execution in progress, please wait...' : 'Operation finished.'}
          </div>
          <Button
            variant={isRunning ? 'outline' : 'default'}
            size="sm"
            onClick={onClose}
            disabled={isRunning}
            className={isRunning ? 'border-slate-700 text-slate-400' : 'bg-rose-600 hover:bg-rose-700 text-white'}
          >
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
