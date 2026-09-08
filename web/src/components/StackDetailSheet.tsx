import React, { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ClusterStack, StackFilesResponse } from '../types';
import { useStackFilesQuery, useStackContainersQuery } from '../hooks/useClusterData';
import { api } from '../services/api';
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
import { OperationLogModal } from './OperationLogModal';
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
  Play,
  Square,
  RotateCw,
  AlertTriangle,
  AlertCircle,
} from 'lucide-react';

interface StackDetailSheetProps {
  stack: ClusterStack | null;
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
  const queryClient = useQueryClient();
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [copiedFile, setCopiedFile] = useState<string | null>(null);

  // Streaming log modal states
  const [logModalOpen, setLogModalOpen] = useState(false);
  const [logTitle, setLogTitle] = useState('');
  const [logSubtitle, setLogSubtitle] = useState('');
  const [logOutput, setLogOutput] = useState('');
  const [logRunning, setLogRunning] = useState(false);
  const [logError, setLogError] = useState<string | null>(null);
  const [logSuccess, setLogSuccess] = useState(false);

  const hostEndpoint = stack?.hostEndpoint || '';
  const stackName = stack?.name || '';
  const isExternal = stack?.source === 'external';

  // For managed stacks: fetch files on mount
  const { data: managedFilesData, isLoading: isLoadingManagedFiles } = useStackFilesQuery(
    hostEndpoint,
    stackName,
    { enabled: Boolean(stackName) && !isExternal }
  );

  // For external stacks: state for manually loaded files
  const [externalFilesData, setExternalFilesData] = useState<StackFilesResponse | null>(null);
  const [isLoadingExternalFiles, setIsLoadingExternalFiles] = useState(false);
  const [externalFilesError, setExternalFilesError] = useState<string | null>(null);

  // Reset external files state when stack changes
  useEffect(() => {
    setExternalFilesData(null);
    setIsLoadingExternalFiles(false);
    setExternalFilesError(null);
  }, [stack?.name, stack?.hostId, stack?.source]);

  const handleLoadExternalFiles = async () => {
    if (!stack) return;
    setIsLoadingExternalFiles(true);
    setExternalFilesError(null);

    try {
      const resp = await api.getStackFiles(stack.name, true, hostEndpoint);
      setExternalFilesData(resp);
    } catch (err: any) {
      setExternalFilesError(err.message || 'Failed to load stack files from host');
    } finally {
      setIsLoadingExternalFiles(false);
    }
  };

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

  const activeFilesData = isExternal ? externalFilesData : managedFilesData;
  const files = activeFilesData?.files || [];
  const filesDir = activeFilesData?.dir;
  const defaultFile = files.find((f) => f.isCompose)?.name || files[0]?.name || '';
  const isExternalFilesLoaded = isExternal && externalFilesData !== null;
  const isLoadingFiles = isExternal ? isLoadingExternalFiles : isLoadingManagedFiles;

  const invalidateStackData = () => {
    queryClient.invalidateQueries({ queryKey: ['cluster-stacks'] });
    queryClient.invalidateQueries({ queryKey: ['cluster-containers'] });
    queryClient.invalidateQueries({ queryKey: ['stack-containers', hostEndpoint, stackName] });
    queryClient.invalidateQueries({ queryKey: ['stack-files', hostEndpoint, stackName] });
  };

  const handleComposeUp = async () => {
    setLogTitle(`Compose Up: ${stack.name}`);
    setLogSubtitle(`Host: ${stack.hostName} | Command: docker compose up -d --remove-orphans`);
    setLogOutput('');
    setLogError(null);
    setLogSuccess(false);
    setLogRunning(true);
    setLogModalOpen(true);

    try {
      await api.composeUp(
        stack.name,
        (chunk) => {
          setLogOutput((prev) => prev + chunk);
        },
        hostEndpoint
      );
      setLogSuccess(true);
      invalidateStackData();
    } catch (err: any) {
      setLogError(err.message || 'Failed to run compose up');
    } finally {
      setLogRunning(false);
    }
  };

  const handleComposeDown = async () => {
    if (stack.is_self) {
      if (!window.confirm('Warning: This stack contains Dokidoki itself. Stopping it will shut down the server. Continue?')) {
        return;
      }
    } else {
      if (!window.confirm(`Are you sure you want to run compose down on stack "${stack.name}"? This will stop and remove its containers.`)) {
        return;
      }
    }

    setLogTitle(`Compose Down: ${stack.name}`);
    setLogSubtitle(`Host: ${stack.hostName} | Command: docker compose down`);
    setLogOutput('');
    setLogError(null);
    setLogSuccess(false);
    setLogRunning(true);
    setLogModalOpen(true);

    try {
      await api.composeDown(
        stack.name,
        (chunk) => {
          setLogOutput((prev) => prev + chunk);
        },
        hostEndpoint
      );
      setLogSuccess(true);
      invalidateStackData();
    } catch (err: any) {
      setLogError(err.message || 'Failed to run compose down');
    } finally {
      setLogRunning(false);
    }
  };

  const handleComposeRestart = async (service?: string) => {
    if (stack.is_self) {
      if (!window.confirm('Warning: This stack contains Dokidoki itself. Restarting it may interrupt your connection. Continue?')) {
        return;
      }
    }

    const titleSuffix = service ? ` (service: ${service})` : '';
    setLogTitle(`Compose Restart: ${stack.name}${titleSuffix}`);
    setLogSubtitle(`Host: ${stack.hostName} | Command: docker compose restart ${service || ''}`.trim());
    setLogOutput('');
    setLogError(null);
    setLogSuccess(false);
    setLogRunning(true);
    setLogModalOpen(true);

    try {
      await api.composeRestart(
        stack.name,
        service,
        (chunk) => {
          setLogOutput((prev) => prev + chunk);
        },
        hostEndpoint
      );
      setLogSuccess(true);
      invalidateStackData();
    } catch (err: any) {
      setLogError(err.message || 'Failed to restart compose stack');
    } finally {
      setLogRunning(false);
    }
  };

  const handleComposePull = async () => {
    setLogTitle(`Compose Pull: ${stack.name}`);
    setLogSubtitle(`Host: ${stack.hostName} | Command: docker compose pull`);
    setLogOutput('');
    setLogError(null);
    setLogSuccess(false);
    setLogRunning(true);
    setLogModalOpen(true);

    try {
      await api.composePull(
        stack.name,
        (chunk) => {
          setLogOutput((prev) => prev + chunk);
        },
        hostEndpoint
      );
      setLogSuccess(true);
      invalidateStackData();
    } catch (err: any) {
      setLogError(err.message || 'Failed to pull compose images');
    } finally {
      setLogRunning(false);
    }
  };

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
                  <Badge variant="outline" className="text-xs bg-slate-800/80 text-slate-300 border-slate-700 font-mono">
                    External
                  </Badge>
                ) : (
                  <>
                    <Badge variant="outline" className="text-xs bg-emerald-950/40 text-emerald-400 border-emerald-800/60 font-mono">
                      Managed
                    </Badge>
                    {stack.pending_import && (
                      <Badge variant="outline" className="bg-amber-950/40 text-amber-400 border-amber-800/60 font-mono text-[11px]">
                        Import Pending
                      </Badge>
                    )}
                  </>
                )}
              </div>
            </div>

            {/* Import Pending Alert Banner */}
            {stack.pending_import && (
              <div className="mt-3 p-3.5 bg-amber-950/40 border border-amber-800/60 rounded-lg text-amber-300 text-xs flex items-start gap-2.5">
                <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
                <div className="leading-relaxed">
                  <span className="font-semibold text-amber-200">Import Pending: </span>
                  This stack was imported into Dokidoki, but its containers are still running from the external directory. Click &apos;Up&apos; or &apos;Restart&apos; to redeploy containers under Dokidoki management.
                </div>
              </div>
            )}

            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400">
              <Badge variant="outline" className="bg-slate-800 text-slate-300 border-slate-700 font-mono flex items-center gap-1">
                <Server className="w-3 h-3 text-slate-400" />
                {stack.hostName}
              </Badge>

              <Badge
                variant="outline"
                className="bg-slate-800 text-slate-300 border-slate-700 font-mono text-[11px]"
              >
                {stack.rollup.running}/{stack.rollup.total} running
              </Badge>

              {stack.is_self && (
                <Badge variant="outline" className="bg-amber-950/40 text-amber-400 border-amber-800/60 font-mono text-[11px] flex items-center gap-1">
                  <AlertTriangle className="w-3 h-3 text-amber-400" />
                  Self Stack
                </Badge>
              )}

              {stack.composePath && (
                <>
                  <span className="text-slate-600">•</span>
                  <span className="truncate max-w-xs text-slate-400 font-mono text-[11px]" title={stack.composePath}>
                    {stack.composePath}
                  </span>
                </>
              )}
            </div>

            {/* Compose Actions Toolbar */}
            <div className="flex flex-wrap items-center gap-2 pt-3">
              <Button
                size="sm"
                variant="outline"
                onClick={handleComposeUp}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors"
              >
                <Play className="w-3.5 h-3.5 mr-1.5 text-emerald-400" />
                Up
              </Button>

              <Button
                size="sm"
                variant="outline"
                onClick={() => handleComposeRestart()}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors"
              >
                <RotateCw className="w-3.5 h-3.5 mr-1.5 text-amber-400" />
                Restart
              </Button>

              <Button
                size="sm"
                variant="outline"
                onClick={handleComposePull}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors"
              >
                <Download className="w-3.5 h-3.5 mr-1.5 text-blue-400" />
                Pull
              </Button>

              <Button
                size="sm"
                variant="outline"
                onClick={handleComposeDown}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors"
              >
                <Square className="w-3.5 h-3.5 mr-1.5 text-rose-400" />
                Down
              </Button>
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

                            {container.service && (
                              <Button
                                size="icon"
                                variant="ghost"
                                title={`Restart service ${container.service}`}
                                className="h-7 w-7 text-slate-400 hover:text-amber-400 hover:bg-slate-800"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleComposeRestart(container.service);
                                }}
                              >
                                <RotateCw className="w-3.5 h-3.5" />
                              </Button>
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
                  Stack Files {isExternal && !isExternalFilesLoaded ? '' : `(${files.length})`}
                </h3>
                <div className="flex items-center gap-2">
                  {filesDir && (
                    <span className="text-[11px] text-slate-500 font-mono truncate max-w-xs flex items-center gap-1">
                      <HardDrive className="w-3 h-3 text-slate-600" />
                      {filesDir}
                    </span>
                  )}
                  {isExternal && isExternalFilesLoaded && (
                    <Button
                      size="sm"
                      onClick={() => setImportDialogOpen(true)}
                      className="h-7 text-xs bg-rose-600 hover:bg-rose-500 text-white font-medium shadow-sm transition-colors"
                    >
                      <Download className="w-3.5 h-3.5 mr-1.5" />
                      Import Stack
                    </Button>
                  )}
                </div>
              </div>

              {isExternal && !isExternalFilesLoaded ? (
                <div className="p-6 bg-slate-950/40 border border-slate-800/80 rounded-xl flex flex-col items-center justify-center text-center space-y-3">
                  <div className="w-10 h-10 rounded-full bg-slate-900 border border-slate-800 flex items-center justify-center text-slate-400">
                    <FileCode className="w-5 h-5 text-slate-400" />
                  </div>
                  <div className="space-y-1 max-w-md">
                    <h4 className="text-sm font-semibold text-slate-200">External Stack Files</h4>
                    <p className="text-xs text-slate-400 leading-relaxed">
                      This stack is running externally on <span className="text-slate-300 font-medium">{stack.hostName}</span>. Dokidoki can read the compose configuration and environment files directly from the host filesystem.
                    </p>
                    {stack.composePath && (
                      <p className="text-[11px] font-mono text-slate-500 truncate pt-1">
                        Host Path: {stack.composePath}
                      </p>
                    )}
                  </div>

                  {externalFilesError && (
                    <div className="w-full max-w-md p-3 bg-rose-950/40 border border-rose-800/60 rounded-lg text-rose-300 text-xs flex items-center gap-2 text-left">
                      <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
                      <span>{externalFilesError}</span>
                    </div>
                  )}

                  <Button
                    onClick={handleLoadExternalFiles}
                    disabled={isLoadingExternalFiles}
                    className="bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium px-4 h-9 shadow-sm"
                  >
                    {isLoadingExternalFiles ? (
                      <>
                        <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                        Loading stack files from host...
                      </>
                    ) : (
                      <>
                        <Download className="w-4 h-4 mr-2" />
                        Load Stack Files
                      </>
                    )}
                  </Button>
                </div>
              ) : isLoadingFiles ? (
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
        loadedFiles={files}
      />

      <OperationLogModal
        isOpen={logModalOpen}
        onClose={() => setLogModalOpen(false)}
        title={logTitle}
        subtitle={logSubtitle}
        output={logOutput}
        isRunning={logRunning}
        error={logError}
        success={logSuccess}
      />
    </>
  );
};
