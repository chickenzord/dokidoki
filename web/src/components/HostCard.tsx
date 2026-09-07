import React from 'react';
import { HostInfo } from '../types';
import { 
  Cpu, 
  Layers, 
  FolderGit2, 
  Box, 
  Server, 
  PlayCircle, 
  PauseCircle, 
  StopCircle 
} from 'lucide-react';

interface HostCardProps {
  hostInfo: HostInfo | null;
  loading: boolean;
  error?: string | null;
}

export const HostCard: React.FC<HostCardProps> = ({ hostInfo, loading, error }) => {
  if (loading && !hostInfo) {
    return (
      <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 animate-pulse">
        <div className="h-6 bg-slate-800 rounded w-1/4 mb-4"></div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="h-16 bg-slate-800/50 rounded-xl"></div>
          <div className="h-16 bg-slate-800/50 rounded-xl"></div>
          <div className="h-16 bg-slate-800/50 rounded-xl"></div>
          <div className="h-16 bg-slate-800/50 rounded-xl"></div>
        </div>
      </div>
    );
  }

  if (error && !hostInfo) {
    return (
      <div className="bg-rose-950/20 border border-rose-900/40 rounded-2xl p-6 text-rose-300">
        <div className="flex items-center gap-2 mb-2 font-semibold">
          <Server className="w-5 h-5 text-rose-400" />
          <span>Failed to connect to host info</span>
        </div>
        <p className="text-sm text-rose-400/80">{error}</p>
      </div>
    );
  }

  if (!hostInfo) return null;

  return (
    <div className="bg-slate-900/80 border border-slate-800/80 rounded-2xl p-6 shadow-xl relative overflow-hidden backdrop-blur-sm">
      {/* Decorative gradient background glow */}
      <div className="absolute -right-20 -top-20 w-64 h-64 bg-rose-500/5 rounded-full blur-3xl pointer-events-none" />

      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
        <div>
          <div className="flex items-center gap-3">
            <h2 className="text-xl font-bold text-white tracking-tight">
              {hostInfo.hostname || 'Docker Host'}
            </h2>
            <span className="px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
              Dokidoki v{hostInfo.dokidokiVersion || '0.1.0'}
            </span>
          </div>
          <p className="text-xs text-slate-400 mt-1 flex items-center gap-1.5 font-mono">
            <FolderGit2 className="w-3.5 h-3.5 text-slate-500" />
            <span>Stacks dir:</span>
            <span className="text-slate-300 bg-slate-800/60 px-2 py-0.5 rounded border border-slate-700/50">
              {hostInfo.stacksDir || '/var/dokidoki/stacks'}
            </span>
          </p>
        </div>

        {/* Quick container stats rollups */}
        <div className="flex items-center gap-3 bg-slate-950/60 border border-slate-800/80 px-4 py-2 rounded-xl">
          <div className="flex items-center gap-1.5 text-xs text-emerald-400" title="Containers Running">
            <PlayCircle className="w-4 h-4" />
            <span className="font-bold">{hostInfo.docker.containersRunning}</span>
            <span className="text-slate-400 hidden sm:inline">running</span>
          </div>
          <div className="w-px h-4 bg-slate-800" />
          <div className="flex items-center gap-1.5 text-xs text-amber-400" title="Containers Paused">
            <PauseCircle className="w-4 h-4" />
            <span className="font-bold">{hostInfo.docker.containersPaused}</span>
            <span className="text-slate-400 hidden sm:inline">paused</span>
          </div>
          <div className="w-px h-4 bg-slate-800" />
          <div className="flex items-center gap-1.5 text-xs text-rose-400" title="Containers Stopped">
            <StopCircle className="w-4 h-4" />
            <span className="font-bold">{hostInfo.docker.containersStopped}</span>
            <span className="text-slate-400 hidden sm:inline">stopped</span>
          </div>
        </div>
      </div>

      {/* Grid of details */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        {/* Docker Version */}
        <div className="bg-slate-950/40 border border-slate-800/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="p-2.5 rounded-lg bg-blue-500/10 text-blue-400 border border-blue-500/20">
            <Box className="w-5 h-5" />
          </div>
          <div className="min-w-0">
            <span className="text-[11px] font-medium text-slate-400 uppercase tracking-wider block">Docker Engine</span>
            <span className="text-sm font-semibold text-slate-100 truncate block font-mono">
              v{hostInfo.docker.engineVersion || 'N/A'}
            </span>
          </div>
        </div>

        {/* Architecture */}
        <div className="bg-slate-950/40 border border-slate-800/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="p-2.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
            <Cpu className="w-5 h-5" />
          </div>
          <div className="min-w-0">
            <span className="text-[11px] font-medium text-slate-400 uppercase tracking-wider block">Architecture</span>
            <span className="text-sm font-semibold text-slate-100 truncate block font-mono">
              {hostInfo.arch || hostInfo.docker.arch || 'unknown'}
            </span>
          </div>
        </div>

        {/* OS */}
        <div className="bg-slate-950/40 border border-slate-800/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="p-2.5 rounded-lg bg-purple-500/10 text-purple-400 border border-purple-500/20">
            <Server className="w-5 h-5" />
          </div>
          <div className="min-w-0">
            <span className="text-[11px] font-medium text-slate-400 uppercase tracking-wider block">Operating System</span>
            <span className="text-sm font-semibold text-slate-100 truncate block">
              {hostInfo.docker.os || hostInfo.os || 'Linux'}
            </span>
          </div>
        </div>

        {/* Total Containers */}
        <div className="bg-slate-950/40 border border-slate-800/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="p-2.5 rounded-lg bg-rose-500/10 text-rose-400 border border-rose-500/20">
            <Layers className="w-5 h-5" />
          </div>
          <div className="min-w-0">
            <span className="text-[11px] font-medium text-slate-400 uppercase tracking-wider block">Total Containers</span>
            <span className="text-sm font-semibold text-slate-100 truncate block font-mono">
              {hostInfo.docker.containers}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
};
