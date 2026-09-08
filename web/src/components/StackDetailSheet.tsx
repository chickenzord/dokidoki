import React, { useState, useEffect, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ClusterStack, StackFilesResponse, ContainerSummary, CreateStackFile } from '../types';
import { useStackFilesQuery, useStackContainersQuery, useStackDetailQuery } from '../hooks/useClusterData';
import { api } from '../services/api';
import { getHostForContainer } from '../lib/utils';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from './ui/sheet';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs';
import { ImportStackDialog } from './ImportStackDialog';
import { OperationLogModal, OperationTask } from './OperationLogModal';
import { CodeEditor } from './CodeEditor';
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
  Edit3,
  Save,
  Plus,
  X,
  Undo2,
  CheckCircle2,
  ExternalLink,
} from 'lucide-react';

interface StackDetailSheetProps {
  stack: ClusterStack | null;
  isOpen: boolean;
  onClose: () => void;
  onSelectContainer?: (containerId: string, hostEndpoint: string, container?: ContainerSummary) => void;
  onStackUpdated?: (stack: ClusterStack) => void;
}

export const StackDetailSheet: React.FC<StackDetailSheetProps> = ({
  stack: initialStack,
  isOpen,
  onClose,
  onSelectContainer,
  onStackUpdated,
}) => {
  const queryClient = useQueryClient();
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [copiedFile, setCopiedFile] = useState<string | null>(null);

  // Editing state for managed stacks
  const [isEditing, setIsEditing] = useState(false);
  const [editedFiles, setEditedFiles] = useState<Record<string, string>>({});
  const [newFiles, setNewFiles] = useState<string[]>([]);
  const [activeFileTab, setActiveFileTab] = useState<string>('');
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState<string | null>(null);
  const [addingFile, setAddingFile] = useState(false);
  const [newFileName, setNewFileName] = useState('');

  // Streaming operation state (isolated in OperationLogModal to prevent re-rendering large sheet on log chunks)
  const [activeOperation, setActiveOperation] = useState<OperationTask | null>(null);

  const hostEndpoint = initialStack?.hostEndpoint || '';
  const stackName = initialStack?.name || '';

  // Query live stack detail to keep modal in sync when operations finish
  const { data: stackDetail } = useStackDetailQuery(
    hostEndpoint,
    stackName,
    isOpen && Boolean(stackName)
  );

  const stack: ClusterStack | null = initialStack
    ? {
        ...initialStack,
        ...(stackDetail ? stackDetail : {}),
      }
    : null;

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

  // Reset external files and editing state when stack changes or sheet closes
  useEffect(() => {
    setExternalFilesData(null);
    setIsLoadingExternalFiles(false);
    setExternalFilesError(null);
    setIsEditing(false);
    setEditedFiles({});
    setNewFiles([]);
    setSaveError(null);
    setSaveSuccess(null);
    setAddingFile(false);
    setNewFileName('');
    setActiveFileTab('');
  }, [stack?.name, stack?.hostId, stack?.source, isOpen]);

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

  const activeFilesData = isExternal ? externalFilesData : managedFilesData;
  const files = activeFilesData?.files || [];
  const filesDir = activeFilesData?.dir;
  const defaultFile = files.find((f) => f.isCompose)?.name || files[0]?.name || '';
  const isExternalFilesLoaded = isExternal && externalFilesData !== null;
  const isLoadingFiles = isExternal ? isLoadingExternalFiles : isLoadingManagedFiles;

  // Keep activeFileTab pointing to default file when not set
  useEffect(() => {
    if (!activeFileTab && defaultFile) {
      setActiveFileTab(defaultFile);
    }
  }, [defaultFile, activeFileTab]);

  const isDirty = useMemo(() => {
    if (newFiles.length > 0) return true;
    for (const f of files) {
      if (editedFiles[f.name] !== undefined && editedFiles[f.name] !== (f.content || '')) {
        return true;
      }
    }
    return false;
  }, [files, editedFiles, newFiles]);

  const editableFileList = useMemo(() => {
    const list: Array<{
      name: string;
      content: string;
      path?: string;
      isNew: boolean;
      isCompose: boolean;
    }> = [];

    files.forEach((f) => {
      list.push({
        name: f.name,
        content: editedFiles[f.name] !== undefined ? editedFiles[f.name] : (f.content || ''),
        path: f.path,
        isNew: false,
        isCompose: f.isCompose,
      });
    });

    newFiles.forEach((name) => {
      const lower = name.toLowerCase();
      list.push({
        name,
        content: editedFiles[name] !== undefined ? editedFiles[name] : '',
        path: filesDir ? `${filesDir}/${name}` : name,
        isNew: true,
        isCompose: lower.endsWith('.yaml') || lower.endsWith('.yml') || lower.includes('compose'),
      });
    });

    return list;
  }, [files, newFiles, editedFiles, filesDir]);

  const handleStartEditing = (fileToFocus?: string) => {
    if (isExternal) return;
    const initialMap: Record<string, string> = {};
    files.forEach((f) => {
      initialMap[f.name] = f.content || '';
    });
    setEditedFiles(initialMap);
    setNewFiles([]);
    setSaveError(null);
    setSaveSuccess(null);
    setAddingFile(false);
    setNewFileName('');
    if (fileToFocus) {
      setActiveFileTab(fileToFocus);
    } else if (!activeFileTab && defaultFile) {
      setActiveFileTab(defaultFile);
    }
    setIsEditing(true);
  };

  const handleCancelEditing = () => {
    if (isDirty) {
      if (!window.confirm('Discard unsaved changes to stack files?')) {
        return;
      }
    }
    setIsEditing(false);
    setEditedFiles({});
    setNewFiles([]);
    setSaveError(null);
    setAddingFile(false);
    setNewFileName('');
    if (defaultFile && !files.some((f) => f.name === activeFileTab)) {
      setActiveFileTab(defaultFile);
    }
  };

  const handleContentChange = (fileName: string, newContent: string) => {
    setEditedFiles((prev) => ({
      ...prev,
      [fileName]: newContent,
    }));
    setSaveSuccess(null);
    setSaveError(null);
  };

  const handleAddNewFile = () => {
    const trimmed = newFileName.trim();
    if (!trimmed) return;

    if (files.some((f) => f.name === trimmed) || newFiles.includes(trimmed)) {
      setSaveError(`File "${trimmed}" already exists in stack.`);
      return;
    }

    if (trimmed.includes('/') || trimmed.includes('\\') || trimmed.includes('..')) {
      setSaveError('Invalid file name: path traversal characters are not allowed.');
      return;
    }

    const defaultContent = trimmed.startsWith('.env')
      ? '# Environment variables\n# KEY=VALUE\n'
      : '';

    setNewFiles((prev) => [...prev, trimmed]);
    setEditedFiles((prev) => ({
      ...prev,
      [trimmed]: defaultContent,
    }));
    setActiveFileTab(trimmed);
    setNewFileName('');
    setAddingFile(false);
    setSaveError(null);
  };

  const handleRemoveNewFile = (fileName: string) => {
    setNewFiles((prev) => prev.filter((n) => n !== fileName));
    setEditedFiles((prev) => {
      const next = { ...prev };
      delete next[fileName];
      return next;
    });
    if (activeFileTab === fileName) {
      const remaining = files.length > 0 ? files[0].name : '';
      setActiveFileTab(remaining);
    }
  };

  const handleSaveStack = async () => {
    if (!stack || isExternal) return;
    setIsSaving(true);
    setSaveError(null);
    setSaveSuccess(null);

    const allFileNames = Array.from(
      new Set([...files.map((f) => f.name), ...newFiles, ...Object.keys(editedFiles)])
    );

    const payloadFiles: CreateStackFile[] = allFileNames.map((name) => ({
      name,
      content: editedFiles[name] !== undefined
        ? editedFiles[name]
        : (files.find((f) => f.name === name)?.content || ''),
    }));

    if (payloadFiles.length === 0) {
      setSaveError('Stack must contain at least one file.');
      setIsSaving(false);
      return;
    }

    try {
      await api.updateStack(
        stack.name,
        {
          name: stack.name,
          files: payloadFiles,
        },
        hostEndpoint
      );

      setSaveSuccess('Stack files saved to host. Running containers were not restarted.');
      setIsEditing(false);
      setEditedFiles({});
      setNewFiles([]);
      invalidateStackData();
      if (onStackUpdated) {
        onStackUpdated(stack);
      }
    } catch (err: any) {
      setSaveError(err.message || 'Failed to save stack files');
    } finally {
      setIsSaving(false);
    }
  };

  const invalidateStackData = () => {
    queryClient.invalidateQueries({ queryKey: ['cluster-stacks'] });
    queryClient.invalidateQueries({ queryKey: ['cluster-containers'] });
    queryClient.invalidateQueries({ queryKey: ['stack-detail', hostEndpoint, stackName] });
    queryClient.invalidateQueries({ queryKey: ['stack-containers', hostEndpoint, stackName] });
    queryClient.invalidateQueries({ queryKey: ['stack-files', hostEndpoint, stackName] });
  };

  const handleComposeUp = () => {
    if (!stack) return;
    setActiveOperation({
      title: `Compose Up: ${stack.name}`,
      subtitle: `Host: ${stack.hostName} | Command: docker compose up -d --remove-orphans`,
      action: (onChunk) => api.composeUp(stack.name, onChunk, hostEndpoint),
      onSuccess: invalidateStackData,
    });
  };

  const handleComposeDown = () => {
    if (!stack) return;
    if (stack.is_self) {
      if (!window.confirm('Warning: This stack contains Dokidoki itself. Stopping it will shut down the server. Continue?')) {
        return;
      }
    } else {
      if (!window.confirm(`Are you sure you want to run compose down on stack "${stack.name}"? This will stop and remove its containers.`)) {
        return;
      }
    }

    setActiveOperation({
      title: `Compose Down: ${stack.name}`,
      subtitle: `Host: ${stack.hostName} | Command: docker compose down`,
      action: (onChunk) => api.composeDown(stack.name, onChunk, hostEndpoint),
      onSuccess: invalidateStackData,
    });
  };

  const handleComposeRestart = (service?: string) => {
    if (!stack) return;
    if (stack.is_self) {
      if (!window.confirm('Warning: This stack contains Dokidoki itself. Restarting it may interrupt your connection. Continue?')) {
        return;
      }
    }

    const titleSuffix = service ? ` (service: ${service})` : '';
    setActiveOperation({
      title: `Compose Restart: ${stack.name}${titleSuffix}`,
      subtitle: `Host: ${stack.hostName} | Command: docker compose restart ${service || ''}`.trim(),
      action: (onChunk) => api.composeRestart(stack.name, service, onChunk, hostEndpoint),
      onSuccess: invalidateStackData,
    });
  };

  const handleComposePull = () => {
    if (!stack) return;
    setActiveOperation({
      title: `Compose Pull: ${stack.name}`,
      subtitle: `Host: ${stack.hostName} | Command: docker compose pull`,
      action: (onChunk) => api.composePull(stack.name, onChunk, hostEndpoint),
      onSuccess: invalidateStackData,
    });
  };

  if (!stack) return null;

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
                  This stack was imported into Dokidoki, but its containers are still running from the external directory. Click &apos;Up&apos; to redeploy containers under Dokidoki management.
                </div>
              </div>
            )}

            {/* Unmanaged Stack Informational Banner */}
            {isExternal && (
              <div className="mt-3 p-3 bg-slate-800/40 border border-slate-700/60 rounded-lg text-slate-300 text-xs flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-slate-400 min-w-0">
                  <AlertCircle className="w-4 h-4 text-slate-400 flex-shrink-0" />
                  <span className="truncate">
                    Unmanaged stack. Compose operations are disabled until imported.
                  </span>
                </div>
                <Button
                  size="sm"
                  onClick={() => setImportDialogOpen(true)}
                  className="h-6 px-2.5 text-xs bg-rose-600 hover:bg-rose-500 text-white font-medium flex-shrink-0"
                >
                  <Download className="w-3 h-3 mr-1" />
                  Import
                </Button>
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
                {stack.rollup.completed ? ` · ${stack.rollup.completed} completed` : ''}
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
                disabled={isExternal}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                title={isExternal ? 'Compose operations are disabled for unmanaged stacks. Import stack to manage.' : undefined}
              >
                <Play className="w-3.5 h-3.5 mr-1.5 text-emerald-400" />
                Up
              </Button>

              <Button
                size="sm"
                variant="outline"
                onClick={() => handleComposeRestart()}
                disabled={isExternal}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                title={isExternal ? 'Compose operations are disabled for unmanaged stacks. Import stack to manage.' : undefined}
              >
                <RotateCw className="w-3.5 h-3.5 mr-1.5 text-amber-400" />
                Restart
              </Button>

              <Button
                size="sm"
                variant="outline"
                onClick={handleComposePull}
                disabled={isExternal}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                title={isExternal ? 'Compose operations are disabled for unmanaged stacks. Import stack to manage.' : undefined}
              >
                <Download className="w-3.5 h-3.5 mr-1.5 text-blue-400" />
                Pull
              </Button>

              <Button
                size="sm"
                variant="outline"
                onClick={handleComposeDown}
                disabled={isExternal}
                className="h-8 text-xs bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                title={isExternal ? 'Compose operations are disabled for unmanaged stacks. Import stack to manage.' : undefined}
              >
                <Square className="w-3.5 h-3.5 mr-1.5 text-rose-400" />
                Down
              </Button>

              {!isExternal && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    if (isEditing) {
                      handleCancelEditing();
                    } else {
                      handleStartEditing();
                    }
                  }}
                  className={`h-8 text-xs transition-colors ${
                    isEditing
                      ? 'bg-rose-950/60 text-rose-300 border-rose-800/60 hover:bg-rose-900/60'
                      : 'bg-slate-800/80 hover:bg-slate-700 text-slate-200 border-slate-700'
                  }`}
                  title={isEditing ? 'Exit editing mode' : 'Edit stack configuration files'}
                >
                  <Edit3 className="w-3.5 h-3.5 mr-1.5 text-rose-400" />
                  {isEditing ? 'Editing' : 'Edit'}
                </Button>
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
                      const isCompleted =
                        container.state === 'exited' &&
                        (container.exit_code === 0 || container.status?.toLowerCase().startsWith('exited (0)'));
                      const cleanName = container.name.replace(/^\//, '');

                      return (
                        <div
                          key={container.id}
                          onClick={() => onSelectContainer?.(container.id, stack.hostEndpoint, container)}
                          className="flex items-center justify-between p-3 hover:bg-slate-800/50 cursor-pointer transition-colors group"
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <span
                              className={`w-2 h-2 rounded-full flex-shrink-0 ${
                                isRunning
                                  ? 'bg-emerald-400 ring-2 ring-emerald-400/20'
                                  : isCompleted
                                  ? 'bg-blue-400 ring-2 ring-blue-400/20'
                                  : 'bg-slate-500'
                              }`}
                              title={isRunning ? 'Running' : isCompleted ? 'Completed (Exit 0)' : container.state}
                            />
                            <div className="min-w-0">
                              <div className="text-xs font-semibold text-slate-200 group-hover:text-white flex items-center gap-1.5 truncate">
                                <span>{cleanName}</span>
                                {container.service && (
                                  <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-400 border border-slate-700/60 font-mono">
                                    {container.service}
                                  </span>
                                )}
                                {isCompleted && (
                                  <span className="text-[10px] px-1.5 py-0.2 rounded bg-blue-950/40 text-blue-300 border border-blue-800/50 font-mono">
                                    completed
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
                                {container.ports.slice(0, 2).map((p, idx) => {
                                  if (p.publicPort) {
                                    const targetHost = getHostForContainer(hostEndpoint);
                                    return (
                                      <a
                                        key={idx}
                                        href={`http://${targetHost}:${p.publicPort}`}
                                        target="_blank"
                                        rel="noreferrer"
                                        onClick={(e) => e.stopPropagation()}
                                        title={`Open service at http://${targetHost}:${p.publicPort}`}
                                        className="inline-flex items-center text-[10px] font-mono tabular-nums px-1.5 py-0.5 rounded bg-slate-800/80 text-slate-300 border border-slate-700/50 hover:text-rose-400 hover:border-rose-500/40 hover:bg-slate-700/80 transition-colors"
                                      >
                                        <span>{p.publicPort}:{p.privatePort}</span>
                                        <ExternalLink className="w-2.5 h-2.5 ml-1 opacity-70" />
                                      </a>
                                    );
                                  }
                                  return (
                                    <span
                                      key={idx}
                                      className="text-[10px] font-mono tabular-nums px-1.5 py-0.5 rounded bg-slate-800/80 text-slate-300 border border-slate-700/50"
                                    >
                                      {p.privatePort}
                                    </span>
                                  );
                                })}
                                {container.ports.length > 2 && (
                                  <span className="text-[10px] text-slate-500 font-mono tabular-nums">
                                    +{container.ports.length - 2}
                                  </span>
                                )}
                              </div>
                            )}

                            {container.service && !isExternal && (
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
                <div className="flex items-center gap-2">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                    <FileCode className="w-3.5 h-3.5 text-slate-400" />
                    Stack Files {isExternal && !isExternalFilesLoaded ? '' : `(${isEditing ? editableFileList.length : files.length})`}
                  </h3>
                  {!isExternal && isEditing && (
                    <Badge variant="outline" className="bg-rose-950/60 text-rose-300 border-rose-800/60 font-mono text-[10px] py-0">
                      Editing
                    </Badge>
                  )}
                  {!isExternal && isEditing && isDirty && (
                    <Badge variant="outline" className="bg-amber-950/60 text-amber-300 border-amber-800/60 font-mono text-[10px] py-0">
                      Unsaved Changes
                    </Badge>
                  )}
                </div>
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
                  {!isExternal && !isEditing && files.length > 0 && (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleStartEditing()}
                      className="h-7 text-xs bg-slate-800/90 hover:bg-slate-700 text-slate-200 border-slate-700 transition-colors flex items-center gap-1.5 shadow-sm"
                    >
                      <Edit3 className="w-3.5 h-3.5 text-rose-400" />
                      Edit Files
                    </Button>
                  )}
                  {!isExternal && isEditing && (
                    <div className="flex items-center gap-2">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={handleCancelEditing}
                        disabled={isSaving}
                        className="h-7 text-xs text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                      >
                        <Undo2 className="w-3.5 h-3.5 mr-1" />
                        Cancel
                      </Button>
                      <Button
                        size="sm"
                        onClick={handleSaveStack}
                        disabled={isSaving || !isDirty}
                        className="h-7 text-xs bg-rose-600 hover:bg-rose-500 text-white font-medium transition-colors shadow-sm disabled:opacity-50 flex items-center gap-1.5"
                      >
                        {isSaving ? (
                          <>
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                            Saving...
                          </>
                        ) : (
                          <>
                            <Save className="w-3.5 h-3.5" />
                            Save Changes
                          </>
                        )}
                      </Button>
                    </div>
                  )}
                </div>
              </div>

              {/* Status alerts */}
              {saveError && (
                <div className="p-3 bg-rose-950/40 border border-rose-800/60 rounded-lg text-rose-300 text-xs flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
                  <span>{saveError}</span>
                </div>
              )}

              {saveSuccess && (
                <div className="p-3 bg-emerald-950/40 border border-emerald-800/60 rounded-lg text-emerald-300 text-xs flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />
                    <span>{saveSuccess}</span>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleComposeUp}
                    className="h-6 px-2 text-[11px] bg-emerald-900/50 hover:bg-emerald-800 text-emerald-200 border-emerald-700/60 flex-shrink-0 flex items-center gap-1"
                  >
                    <Play className="w-3 h-3 text-emerald-400" />
                    Run Up now
                  </Button>
                </div>
              )}

              {/* Editing Controls & Add File Bar */}
              {!isExternal && isEditing && (
                <div className="p-3 bg-slate-950/70 border border-slate-800/90 rounded-lg space-y-2">
                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                    <div className="flex items-center gap-2 text-slate-400">
                      <span className="text-slate-300 font-medium">Stack Configuration Editor</span>
                      <span className="text-slate-600">•</span>
                      <span className="text-[11px] text-slate-400">
                        Editing is just editing; changes are saved to disk without automatically updating running containers.
                      </span>
                    </div>

                    {!addingFile ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setAddingFile(true)}
                        className="h-6 px-2 text-[11px] bg-slate-900 hover:bg-slate-800 text-slate-300 border-slate-700 flex items-center gap-1"
                      >
                        <Plus className="w-3 h-3 text-rose-400" />
                        Add File
                      </Button>
                    ) : null}
                  </div>

                  {addingFile && (
                    <div className="flex items-center gap-2 pt-1 border-t border-slate-800/80">
                      <Input
                        value={newFileName}
                        onChange={(e) => setNewFileName(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') handleAddNewFile();
                          if (e.key === 'Escape') {
                            setAddingFile(false);
                            setNewFileName('');
                          }
                        }}
                        placeholder="e.g. .env or docker-compose.override.yaml"
                        className="h-7 text-xs bg-slate-900 border-slate-700 text-slate-200 font-mono flex-1 focus:ring-rose-500"
                        autoFocus
                      />
                      <Button
                        size="sm"
                        onClick={handleAddNewFile}
                        disabled={!newFileName.trim()}
                        className="h-7 px-2.5 text-xs bg-rose-600 hover:bg-rose-500 text-white"
                      >
                        Add
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => {
                          setAddingFile(false);
                          setNewFileName('');
                        }}
                        className="h-7 px-2 text-xs text-slate-400 hover:text-white"
                      >
                        Cancel
                      </Button>
                    </div>
                  )}
                </div>
              )}

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
              ) : files.length === 0 && (!isEditing || editableFileList.length === 0) ? (
                <div className="p-6 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-500 text-xs text-center">
                  No configuration files found in stack directory
                </div>
              ) : isEditing ? (
                /* Editing Mode Tabs & Editor */
                <Tabs
                  value={activeFileTab || (editableFileList[0]?.name || '')}
                  onValueChange={setActiveFileTab}
                  className="w-full"
                >
                  <TabsList className="bg-slate-950/80 border border-slate-800/80 p-0.5 h-auto flex flex-wrap gap-1">
                    {editableFileList.map((file) => {
                      const isModified =
                        editedFiles[file.name] !== undefined &&
                        (file.isNew || editedFiles[file.name] !== (files.find((f) => f.name === file.name)?.content || ''));

                      return (
                        <TabsTrigger
                          key={file.name}
                          value={file.name}
                          className="text-xs font-mono px-3 py-1.5 data-[state=active]:bg-slate-800 data-[state=active]:text-white text-slate-400 flex items-center gap-1.5"
                        >
                          <span>{file.name}</span>
                          {file.isCompose && (
                            <span className="w-1.5 h-1.5 rounded-full bg-rose-400" title="Compose file" />
                          )}
                          {isModified && (
                            <span className="w-1.5 h-1.5 rounded-full bg-amber-400" title="Unsaved modifications" />
                          )}
                          {file.isNew && (
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                handleRemoveNewFile(file.name);
                              }}
                              className="ml-1 text-slate-500 hover:text-rose-400"
                              title="Remove new file"
                            >
                              <X className="w-3 h-3" />
                            </button>
                          )}
                        </TabsTrigger>
                      );
                    })}
                  </TabsList>

                  {editableFileList.map((file) => {
                    const isCopied = copiedFile === file.name;
                    const isModified =
                      editedFiles[file.name] !== undefined &&
                      (file.isNew || editedFiles[file.name] !== (files.find((f) => f.name === file.name)?.content || ''));

                    return (
                      <TabsContent key={file.name} value={file.name} className="mt-2 outline-none">
                        <div className="relative border border-slate-800 rounded-lg bg-slate-950 overflow-hidden">
                          <div className="flex items-center justify-between px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/80 text-[11px] text-slate-400">
                            <div className="flex items-center gap-2">
                              <span className="font-mono text-slate-200">{file.path || file.name}</span>
                              {file.isNew && (
                                <Badge variant="outline" className="text-[10px] bg-blue-950/40 text-blue-300 border-blue-800/50 font-mono py-0">
                                  new file
                                </Badge>
                              )}
                              {isModified && !file.isNew && (
                                <Badge variant="outline" className="text-[10px] bg-amber-950/40 text-amber-300 border-amber-800/50 font-mono py-0">
                                  modified
                                </Badge>
                              )}
                            </div>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => handleCopy(file.content, file.name)}
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

                          <CodeEditor
                            value={file.content}
                            onChange={(newContent) => handleContentChange(file.name, newContent)}
                            filename={file.name}
                            readOnly={false}
                            minHeight="280px"
                            maxHeight="520px"
                          />
                        </div>
                      </TabsContent>
                    );
                  })}
                </Tabs>
              ) : (
                /* Read-Only Mode Tabs & Viewer with Syntax Highlighting */
                <Tabs
                  value={activeFileTab || defaultFile}
                  onValueChange={setActiveFileTab}
                  className="w-full"
                >
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
                    const isCopied = copiedFile === file.name;

                    return (
                      <TabsContent key={file.name} value={file.name} className="mt-2 outline-none">
                        <div className="relative border border-slate-800 rounded-lg bg-slate-950 overflow-hidden">
                          <div className="flex items-center justify-between px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/80 text-[11px] text-slate-400">
                            <span className="font-mono text-slate-300">{file.path}</span>
                            <div className="flex items-center gap-1.5">
                              {!isExternal && (
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => handleStartEditing(file.name)}
                                  className="h-6 px-2 text-[11px] text-slate-400 hover:text-white hover:bg-slate-800 flex items-center gap-1"
                                >
                                  <Edit3 className="w-3 h-3 text-rose-400" />
                                  Edit
                                </Button>
                              )}
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
                          </div>

                          <CodeEditor
                            value={file.content || ''}
                            filename={file.name}
                            readOnly={true}
                            minHeight="180px"
                            maxHeight="480px"
                          />
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
        onClose={() => {
          setImportDialogOpen(false);
          invalidateStackData();
        }}
        onImportSuccess={(createdSummary) => {
          invalidateStackData();
          if (onStackUpdated && stack) {
            onStackUpdated({
              ...stack,
              ...createdSummary,
              source: 'managed',
              pending_import: true,
            });
          }
        }}
        stack={stack}
        loadedFiles={files}
      />

      <OperationLogModal
        isOpen={Boolean(activeOperation)}
        onClose={() => setActiveOperation(null)}
        operation={activeOperation}
      />
    </>
  );
};
