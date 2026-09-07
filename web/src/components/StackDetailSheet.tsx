import React, { useState } from 'react';
import { FleetStack } from '../types';
import { useStackFilesQuery, useStackContainersQuery } from '../hooks/useFleetData';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from './ui/sheet';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs';
import { ImportStackDialog } from './ImportStackDialog';
import {
  Server,
  Layers,
  FileCode,
  Copy,
  Check,
  Download,
  Loader2,
  Box,
  ArrowUpRight,
  HardDrive,
} from 'lucide-react';

interface StackDetailSheetProps {
  stack: FleetStack | null;
  isOpen: boolean;
  onClose: () => void;
  onSelectContainer?: (containerId: string, hostEndpoint: string) => void;
}

export const StackDetailSheet: React.FC<StackDetailSheetProps> = ({
  stack,
  isOpen,
  onClose,
  onSelectContainer,
}) => {
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [copiedFile, setCopiedFile] = useState<string | null>(null);

  const hostEndpoint = stack?.hostEndpoint || '';
  const stackName = stack?.name || '';

  const { data: filesData, isLoading: isLoadingFiles } = useStackFilesQuery(
    hostEndpoint,
    stackName
  );

  const { data: containers = [], isLoading: isLoadingContainers } = useStackContainersQuery(
    hostEndpoint,
    stackName
  );

  const handleCopy = (content: string, filename: string) => {
    navigator.clipboard.writeText(content);
    setCopiedFile(filename);
    setTimeout(() => setCopiedFile(null), 2000);
  };

  if (!stack) return null;

  const files = filesData?.files || [];
  const defaultFile = files.find((f) => f.isCompose)?.name || files[0]?.name || '';

  return (
    <>
      <Sheet open={isOpen} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="w-full sm:max-w-2xl md:max-w-3xl overflow-y-auto bg-slate-900 border-l border-slate-800 p-6 text-slate-100">
          <SheetHeader className="space-y-2 pb-4 border-b border-slate-800">
            <div className="flex flex-wrap items-center justify-between gap-3 pr-8">
              <div className="flex items-center gap-2">
                <SheetTitle className="text-xl font-bold text-white tracking-tight flex items-center gap-2">
                  <Layers className="w-5 h-5 text-rose-500" />
                  {stack.name}
                </SheetTitle>
              </div>

              <div className="flex items-center gap-2">
                {stack.source === 'external' ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setImportDialogOpen(true)}
                    className="h-7 text-xs bg-rose-600/10 hover:bg-rose-600 text-rose-400 hover:text-white border-rose-500/30 transition-colors"
                  >
                    <Download className="w-3.5 h-3.5 mr-1.5" />
                    Import Stack
                  </Button>
                ) : (
                  <Badge variant="outline" className="text-xs bg-emerald-950/40 text-emerald-400 border-emerald-800/60 font-mono">
                    Managed
                  </Badge>
                )}
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400">
              <Badge variant="outline" className="bg-slate-800 text-slate-300 border-slate-700 font-mono flex items-center gap-1">
                <Server className="w-3 h-3 text-slate-400" />
                {stack.hostName}
              </Badge>

              <span className="text-slate-600">•</span>

              <span className="font-mono text-slate-300">
                {stack.rollup.running}/{stack.rollup.total} containers running
              </span>

              {stack.composePath && (
                <>
                  <span className="text-slate-600">•</span>
                  <span className="truncate max-w-xs text-slate-400 font-mono text-[11px]" title={stack.composePath}>
                    {stack.composePath}
                  </span>
                </>
              )}
            </div>
          </SheetHeader>

          <div className="space-y-6 py-4">
            {/* Section 1: Containers Table */}
            <div className="space-y-2.5">
              <div className="flex items-center justify-between">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                  <Box className="w-3.5 h-3.5 text-slate-400" />
                  Containers ({containers.length || stack.services.length})
                </h3>
              </div>

              {isLoadingContainers ? (
                <div className="p-4 bg-slate-950/40 border border-slate-800/60 rounded-lg flex items-center justify-center gap-2 text-slate-400 text-xs">
                  <Loader2 className="w-4 h-4 animate-spin text-rose-500" />
                  Loading containers...
                </div>
              ) : containers.length === 0 ? (
                <div className="p-4 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-500 text-xs text-center">
                  No active containers found for this stack
                </div>
              ) : (
                <div className="border border-slate-800 rounded-lg overflow-hidden bg-slate-950/40">
                  <div className="divide-y divide-slate-800/80">
                    {containers.map((container) => {
                      const isRunning = container.state === 'running';
                      const cleanName = container.name.replace(/^\//, '');

                      return (
                        <div
                          key={container.id}
                          onClick={() => onSelectContainer?.(container.id, stack.hostEndpoint)}
                          className="flex items-center justify-between p-3 hover:bg-slate-800/50 cursor-pointer transition-colors group"
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <span
                              className={`w-2 h-2 rounded-full flex-shrink-0 ${
                                isRunning ? 'bg-emerald-400 ring-2 ring-emerald-400/20' : 'bg-slate-500'
                              }`}
                            />
                            <div className="min-w-0">
                              <div className="text-xs font-semibold text-slate-200 group-hover:text-white flex items-center gap-1.5 truncate">
                                <span>{cleanName}</span>
                                {container.service && (
                                  <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-400 border border-slate-700/60 font-mono">
                                    {container.service}
                                  </span>
                                )}
                              </div>
                              <div className="text-[11px] text-slate-400 font-mono truncate mt-0.5">
                                {container.image}
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-2 flex-shrink-0 pl-2">
                            {container.ports && container.ports.length > 0 && (
                              <div className="hidden sm:flex items-center gap-1">
                                {container.ports.slice(0, 2).map((p, idx) => (
                                  <span
                                    key={idx}
                                    className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800/80 text-slate-300 border border-slate-700/50"
                                  >
                                    {p.publicPort ? `${p.publicPort}:${p.privatePort}` : p.privatePort}
                                  </span>
                                ))}
                                {container.ports.length > 2 && (
                                  <span className="text-[10px] text-slate-500 font-mono">
                                    +{container.ports.length - 2}
                                  </span>
                                )}
                              </div>
                            )}
                            <ArrowUpRight className="w-3.5 h-3.5 text-slate-500 group-hover:text-slate-300 transition-colors" />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>

            {/* Section 2: Stack Directory Files */}
            <div className="space-y-2.5">
              <div className="flex items-center justify-between">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                  <FileCode className="w-3.5 h-3.5 text-slate-400" />
                  Stack Files ({files.length})
                </h3>
                {filesData?.dir && (
                  <span className="text-[11px] text-slate-500 font-mono truncate max-w-xs flex items-center gap-1">
                    <HardDrive className="w-3 h-3 text-slate-600" />
                    {filesData.dir}
                  </span>
                )}
              </div>

              {isLoadingFiles ? (
                <div className="p-8 bg-slate-950/40 border border-slate-800/60 rounded-lg flex items-center justify-center gap-2 text-slate-400 text-xs">
                  <Loader2 className="w-4 h-4 animate-spin text-rose-500" />
                  Loading stack files...
                </div>
              ) : files.length === 0 ? (
                <div className="p-6 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-500 text-xs text-center">
                  No configuration files found in stack directory
                </div>
              ) : (
                <Tabs defaultValue={defaultFile} className="w-full">
                  <TabsList className="bg-slate-950/80 border border-slate-800/80 p-0.5 h-auto flex flex-wrap gap-1">
                    {files.map((file) => (
                      <TabsTrigger
                        key={file.name}
                        value={file.name}
                        className="text-xs font-mono px-3 py-1.5 data-[state=active]:bg-slate-800 data-[state=active]:text-white text-slate-400"
                      >
                        {file.name}
                        {file.isCompose && (
                          <span className="ml-1.5 w-1.5 h-1.5 rounded-full bg-rose-400" />
                        )}
                      </TabsTrigger>
                    ))}
                  </TabsList>

                  {files.map((file) => {
                    const lines = (file.content || '').split('\n');
                    const isCopied = copiedFile === file.name;

                    return (
                      <TabsContent key={file.name} value={file.name} className="mt-2 outline-none">
                        <div className="relative border border-slate-800 rounded-lg bg-slate-950/80 overflow-hidden">
                          <div className="flex items-center justify-between px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/80 text-[11px] text-slate-400">
                            <span className="font-mono">{file.path}</span>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => handleCopy(file.content || '', file.name)}
                              className="h-6 px-2 text-[11px] text-slate-400 hover:text-white hover:bg-slate-800 flex items-center gap-1"
                            >
                              {isCopied ? (
                                <>
                                  <Check className="w-3 h-3 text-emerald-400" />
                                  Copied
                                </>
                              ) : (
                                <>
                                  <Copy className="w-3 h-3" />
                                  Copy
                                </>
                              )}
                            </Button>
                          </div>

                          <div className="p-3 max-h-96 overflow-y-auto font-mono text-xs text-slate-200 leading-relaxed select-text">
                            {lines.map((line, idx) => (
                              <div key={idx} className="table-row hover:bg-slate-900/40">
                                <span className="table-cell pr-4 text-slate-600 select-none text-right w-8">
                                  {idx + 1}
                                </span>
                                <span className="table-cell whitespace-pre">{line}</span>
                              </div>
                            ))}
                          </div>
                        </div>
                      </TabsContent>
                    );
                  })}
                </Tabs>
              )}
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <ImportStackDialog
        isOpen={importDialogOpen}
        onClose={() => setImportDialogOpen(false)}
        stack={stack}
      />
    </>
  );
};
