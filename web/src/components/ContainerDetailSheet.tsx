import React, { useState } from 'react';
import { FleetContainer } from '../types';
import { useContainerInspectQuery } from '../hooks/useFleetData';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from './ui/sheet';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { GenerateComposeDialog } from './GenerateComposeDialog';
import {
  Server,
  Box,
  Layers,
  Sparkles,
  ChevronDown,
  ChevronRight,
  HardDrive,
  Network,
  Terminal,
  Clock,
  Tag,
  Copy,
  Check,
  Loader2,
} from 'lucide-react';

interface ContainerDetailSheetProps {
  container: FleetContainer | null;
  isOpen: boolean;
  onClose: () => void;
}

export const ContainerDetailSheet: React.FC<ContainerDetailSheetProps> = ({
  container,
  isOpen,
  onClose,
}) => {
  const [generateDialogOpen, setGenerateDialogOpen] = useState(false);
  const [rawInspectOpen, setRawInspectOpen] = useState(false);
  const [copiedInspect, setCopiedInspect] = useState(false);

  const hostEndpoint = container?.hostEndpoint || '';
  const containerId = container?.id || '';

  const { data: inspectData, isLoading: isLoadingInspect } = useContainerInspectQuery(
    hostEndpoint,
    containerId
  );

  if (!container) return null;

  const cleanName = container.name.replace(/^\//, '');
  const isRunning = container.state === 'running';
  const isStandalone = !container.stack;

  const envVars = inspectData?.Config?.Env || [];
  const mounts = inspectData?.Mounts || [];
  const command = inspectData?.Config?.Cmd?.join(' ') || (inspectData?.Path ? `${inspectData.Path} ${inspectData.Args?.join(' ') || ''}` : null);
  const workDir = inspectData?.Config?.WorkingDir;
  const createdDate = container.created
    ? new Date(container.created * 1000).toLocaleString()
    : (inspectData?.Created ? new Date(inspectData.Created).toLocaleString() : 'Unknown');

  const handleCopyInspect = () => {
    if (!inspectData) return;
    navigator.clipboard.writeText(JSON.stringify(inspectData, null, 2));
    setCopiedInspect(true);
    setTimeout(() => setCopiedInspect(false), 2000);
  };

  return (
    <>
      <Sheet open={isOpen} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="w-full sm:max-w-2xl md:max-w-3xl overflow-y-auto bg-slate-900 border-l border-slate-800 p-6 text-slate-100">
          <SheetHeader className="space-y-2 pb-4 border-b border-slate-800">
            <div className="flex flex-wrap items-center justify-between gap-3 pr-8">
              <div className="flex items-center gap-2">
                <SheetTitle className="text-xl font-bold text-white tracking-tight flex items-center gap-2">
                  <Box className="w-5 h-5 text-rose-500" />
                  {cleanName}
                </SheetTitle>
              </div>

              <div className="flex items-center gap-2">
                {isStandalone ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setGenerateDialogOpen(true)}
                    className="h-7 text-xs bg-amber-600/10 hover:bg-amber-600 text-amber-400 hover:text-white border-amber-500/30 transition-colors"
                  >
                    <Sparkles className="w-3.5 h-3.5 mr-1.5" />
                    Manage as Stack
                  </Button>
                ) : (
                  <Badge variant="outline" className="text-xs bg-blue-950/40 text-blue-400 border-blue-800/60 font-mono flex items-center gap-1">
                    <Layers className="w-3 h-3" />
                    {container.stack}
                  </Badge>
                )}
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400">
              <Badge variant="outline" className="bg-slate-800 text-slate-300 border-slate-700 font-mono flex items-center gap-1">
                <Server className="w-3 h-3 text-slate-400" />
                {container.hostName}
              </Badge>

              <Badge
                variant="outline"
                className={`text-[11px] font-mono capitalize ${
                  isRunning
                    ? 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60'
                    : 'bg-slate-800 text-slate-400 border-slate-700'
                }`}
              >
                {container.state}
              </Badge>

              <span className="text-slate-600">•</span>

              <span className="font-mono text-slate-400 text-[11px] truncate max-w-xs" title={container.id}>
                {container.id.substring(0, 12)}
              </span>
            </div>
          </SheetHeader>

          <div className="space-y-6 py-4">
            {/* Overview Attributes */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="p-3 rounded-lg bg-slate-950/40 border border-slate-800/60 space-y-1">
                <div className="text-[11px] font-medium uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                  <Tag className="w-3 h-3 text-slate-500" />
                  Image
                </div>
                <div className="text-xs font-mono text-slate-200 break-all select-all">
                  {container.image}
                </div>
              </div>

              <div className="p-3 rounded-lg bg-slate-950/40 border border-slate-800/60 space-y-1">
                <div className="text-[11px] font-medium uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                  <Clock className="w-3 h-3 text-slate-500" />
                  Created
                </div>
                <div className="text-xs font-mono text-slate-200">
                  {createdDate}
                </div>
              </div>

              {command && (
                <div className="sm:col-span-2 p-3 rounded-lg bg-slate-950/40 border border-slate-800/60 space-y-1">
                  <div className="text-[11px] font-medium uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                    <Terminal className="w-3 h-3 text-slate-500" />
                    Command
                  </div>
                  <div className="text-xs font-mono text-slate-300 bg-slate-900/80 p-2 rounded border border-slate-800/80 break-all select-all">
                    {command}
                  </div>
                </div>
              )}

              {workDir && (
                <div className="sm:col-span-2 p-3 rounded-lg bg-slate-950/40 border border-slate-800/60 space-y-1">
                  <div className="text-[11px] font-medium uppercase tracking-wider text-slate-400">
                    Working Directory
                  </div>
                  <div className="text-xs font-mono text-slate-300">
                    {workDir}
                  </div>
                </div>
              )}
            </div>

            {/* Ports Section */}
            <div className="space-y-2">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                <Network className="w-3.5 h-3.5 text-slate-400" />
                Port Mappings
              </h3>

              {!container.ports || container.ports.length === 0 ? (
                <div className="p-3 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-500 text-xs">
                  No exposed ports
                </div>
              ) : (
                <div className="border border-slate-800 rounded-lg overflow-hidden bg-slate-950/40">
                  <table className="w-full text-xs text-left">
                    <thead className="bg-slate-900/60 text-slate-400 text-[11px] font-mono border-b border-slate-800">
                      <tr>
                        <th className="p-2.5">Type</th>
                        <th className="p-2.5">Private Port</th>
                        <th className="p-2.5">Public Host Port</th>
                        <th className="p-2.5">Host IP</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60 font-mono text-slate-300">
                      {container.ports.map((p, idx) => (
                        <tr key={idx} className="hover:bg-slate-800/30">
                          <td className="p-2.5 uppercase text-slate-400 text-[10px]">{p.type}</td>
                          <td className="p-2.5">{p.privatePort}</td>
                          <td className="p-2.5 text-rose-400">{p.publicPort ? p.publicPort : '—'}</td>
                          <td className="p-2.5 text-slate-400">{p.ip || '0.0.0.0'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            {/* Mounts / Volumes */}
            <div className="space-y-2">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                <HardDrive className="w-3.5 h-3.5 text-slate-400" />
                Mounts & Volumes ({mounts.length})
              </h3>

              {mounts.length === 0 ? (
                <div className="p-3 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-500 text-xs">
                  No mounted volumes
                </div>
              ) : (
                <div className="border border-slate-800 rounded-lg overflow-hidden bg-slate-950/40">
                  <div className="divide-y divide-slate-800/60">
                    {mounts.map((m, idx) => (
                      <div key={idx} className="p-2.5 space-y-1 text-xs">
                        <div className="flex items-center justify-between text-[11px]">
                          <span className="font-mono text-slate-400 uppercase text-[10px] bg-slate-900 px-1.5 py-0.5 rounded border border-slate-800">
                            {m.Type || 'bind'}
                          </span>
                          <span className="text-[10px] text-slate-500 font-mono">
                            {m.RW ? 'Read/Write' : 'Read Only'}
                          </span>
                        </div>
                        <div className="font-mono text-slate-300 text-[11px] truncate" title={m.Source}>
                          <span className="text-slate-500">Host:</span> {m.Source}
                        </div>
                        <div className="font-mono text-rose-400 text-[11px] truncate" title={m.Destination}>
                          <span className="text-slate-500">Container:</span> {m.Destination}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Environment Variables */}
            <div className="space-y-2">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-1.5">
                Environment Variables ({envVars.length})
              </h3>

              {isLoadingInspect ? (
                <div className="p-3 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-400 text-xs flex items-center gap-2">
                  <Loader2 className="w-3.5 h-3.5 animate-spin" /> Loading configuration...
                </div>
              ) : envVars.length === 0 ? (
                <div className="p-3 bg-slate-950/40 border border-slate-800/60 rounded-lg text-slate-500 text-xs">
                  No environment variables defined
                </div>
              ) : (
                <div className="border border-slate-800 rounded-lg overflow-hidden bg-slate-950/40 max-h-56 overflow-y-auto">
                  <div className="divide-y divide-slate-800/60 font-mono text-xs">
                    {envVars.map((env, idx) => {
                      const splitIdx = env.indexOf('=');
                      const key = splitIdx !== -1 ? env.substring(0, splitIdx) : env;
                      const val = splitIdx !== -1 ? env.substring(splitIdx + 1) : '';
                      return (
                        <div key={idx} className="p-2 flex flex-col sm:flex-row sm:items-baseline gap-1 hover:bg-slate-900/40">
                          <span className="text-slate-400 font-semibold sm:w-1/3 truncate" title={key}>
                            {key}
                          </span>
                          <span className="text-slate-200 sm:w-2/3 break-all text-[11px]">
                            {val}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>

            {/* Collapsible Raw Inspect */}
            <div className="border border-slate-800 rounded-lg overflow-hidden bg-slate-950/60">
              <button
                type="button"
                onClick={() => setRawInspectOpen(!rawInspectOpen)}
                className="w-full flex items-center justify-between p-3 text-xs font-semibold text-slate-400 hover:text-slate-200 hover:bg-slate-900/50 transition-colors"
              >
                <span className="flex items-center gap-1.5">
                  {rawInspectOpen ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                  Raw Container Inspect JSON
                </span>
                {inspectData && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleCopyInspect();
                    }}
                    className="h-6 px-2 text-[11px] text-slate-400 hover:text-white"
                  >
                    {copiedInspect ? (
                      <>
                        <Check className="w-3 h-3 text-emerald-400 mr-1" /> Copied
                      </>
                    ) : (
                      <>
                        <Copy className="w-3 h-3 mr-1" /> Copy JSON
                      </>
                    )}
                  </Button>
                )}
              </button>

              {rawInspectOpen && (
                <div className="p-3 border-t border-slate-800 bg-slate-950">
                  {isLoadingInspect ? (
                    <div className="text-xs text-slate-500 py-2">Loading inspect payload...</div>
                  ) : inspectData ? (
                    <pre className="text-[11px] font-mono text-slate-300 max-h-80 overflow-auto whitespace-pre-wrap break-all select-all">
                      {JSON.stringify(inspectData, null, 2)}
                    </pre>
                  ) : (
                    <div className="text-xs text-slate-500 py-2">No inspect data available</div>
                  )}
                </div>
              )}
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <GenerateComposeDialog
        isOpen={generateDialogOpen}
        onClose={() => setGenerateDialogOpen(false)}
        container={container}
      />
    </>
  );
};
