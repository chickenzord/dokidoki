import React, { useState, useEffect, useRef } from 'react';
import { Node, PingResponse } from '../types';
import { api } from '../services/api';
import { 
  Activity, 
  Server, 
  ChevronDown, 
  Check, 
  RefreshCw, 
  Radio, 
  HardDrive,
  Globe
} from 'lucide-react';

interface NavbarProps {
  activeNode: Node | null;
  nodes: Node[];
  onSelectNode: (node: Node | null) => void;
  onRefresh: () => void;
  isLoading: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  activeNode,
  nodes,
  onSelectNode,
  onRefresh,
  isLoading,
}) => {
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [hostStatus, setHostStatus] = useState<PingResponse>({ status: 'checking' });
  const [lastPingTime, setLastPingTime] = useState<Date>(new Date());
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as HTMLElement)) {
        setDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Periodic host ping (every 10s)
  useEffect(() => {
    let isMounted = true;
    const checkPing = async () => {
      try {
        const res = await api.ping();
        if (isMounted) {
          setHostStatus(res);
          setLastPingTime(new Date());
        }
      } catch (err: any) {
        if (isMounted) {
          setHostStatus({ status: 'error', error: err.message });
          setLastPingTime(new Date());
        }
      }
    };

    checkPing();
    const interval = setInterval(checkPing, 10000);
    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, [activeNode]);

  const selfNode = nodes.find(n => n.is_self);
  const currentNodeName = activeNode ? activeNode.name : (selfNode ? `${selfNode.name} (Local)` : 'Local Node');
  const isHostOk = hostStatus.status === 'ok';

  return (
    <header className="sticky top-0 z-50 bg-slate-900/90 backdrop-blur-md border-b border-slate-800">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo and Brand */}
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-rose-600 via-pink-500 to-rose-400 flex items-center justify-center shadow-lg shadow-rose-950/50">
              <Activity className="w-6 h-6 text-white" />
            </div>
            <span className="font-extrabold text-xl tracking-tight text-white">
              Dokidoki
            </span>
          </div>

          {/* Right side controls */}
          <div className="flex items-center gap-4">
            {/* Host Status Badge */}
            <div 
              className={`flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium border transition-colors ${
                isHostOk 
                  ? 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60 shadow-sm shadow-emerald-950/30' 
                  : 'bg-rose-950/40 text-rose-400 border-rose-800/60 shadow-sm shadow-rose-950/30'
              }`}
              title={`Docker Ping Status: ${hostStatus.status} (Checked: ${lastPingTime.toLocaleTimeString()})`}
            >
              <span className="relative flex h-2 w-2">
                {isHostOk && (
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                )}
                <span className={`relative inline-flex rounded-full h-2 w-2 ${isHostOk ? 'bg-emerald-500' : 'bg-rose-500'}`}></span>
              </span>
              <span className="capitalize">{isHostOk ? 'Host Online' : 'Host Offline'}</span>
            </div>

            {/* Node Switcher Dropdown */}
            <div className="relative" ref={dropdownRef}>
              <button
                type="button"
                onClick={() => setDropdownOpen(!dropdownOpen)}
                className="flex items-center gap-2.5 px-3.5 py-1.5 rounded-lg bg-slate-800/90 hover:bg-slate-800 text-slate-200 border border-slate-700/80 text-sm font-medium transition-all shadow-sm focus:outline-none focus:ring-2 focus:ring-rose-500/50"
              >
                <Server className="w-4 h-4 text-rose-400" />
                <span className="max-w-[150px] truncate">{currentNodeName}</span>
                <ChevronDown className={`w-4 h-4 text-slate-400 transition-transform ${dropdownOpen ? 'rotate-180' : ''}`} />
              </button>

              {dropdownOpen && (
                <div className="absolute right-0 mt-2 w-72 rounded-xl bg-slate-900 border border-slate-800 shadow-2xl shadow-black/80 py-2 z-50 animate-in fade-in slide-in-from-top-2 duration-150">
                  <div className="px-3.5 py-2 border-b border-slate-800 flex items-center justify-between">
                    <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                      Cluster Nodes ({nodes.length || 1})
                    </span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onRefresh();
                      }}
                      className="text-xs text-slate-400 hover:text-slate-200 flex items-center gap-1"
                      title="Refresh nodes"
                    >
                      <RefreshCw className={`w-3 h-3 ${isLoading ? 'animate-spin' : ''}`} />
                      Refresh
                    </button>
                  </div>

                  <div className="max-h-64 overflow-y-auto py-1">
                    {/* Option: Local Node (Default) */}
                    <button
                      type="button"
                      onClick={() => {
                        onSelectNode(null);
                        setDropdownOpen(false);
                      }}
                      className="w-full text-left px-3.5 py-2.5 flex items-center justify-between hover:bg-slate-800/80 transition-colors"
                    >
                      <div className="flex items-center gap-2.5 min-w-0">
                        <HardDrive className="w-4 h-4 text-slate-400 flex-shrink-0" />
                        <div className="min-w-0">
                          <div className="text-sm font-medium text-slate-200 flex items-center gap-1.5">
                            <span>{selfNode ? selfNode.name : 'Local Daemon'}</span>
                            <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-300 font-mono border border-slate-700">
                              Local
                            </span>
                          </div>
                          <span className="text-xs text-slate-400">Direct server origin</span>
                        </div>
                      </div>
                      {activeNode === null && <Check className="w-4 h-4 text-rose-400 flex-shrink-0" />}
                    </button>

                    {/* Remote Nodes list */}
                    {nodes
                      .filter(n => !n.is_self)
                      .map((node) => {
                        const isSelected = activeNode?.id === node.id;
                        const isAlive = node.status === 'alive';
                        return (
                          <button
                            key={node.id}
                            type="button"
                            onClick={() => {
                              onSelectNode(node);
                              setDropdownOpen(false);
                            }}
                            className="w-full text-left px-3.5 py-2.5 flex items-center justify-between hover:bg-slate-800/80 transition-colors"
                          >
                            <div className="flex items-center gap-2.5 min-w-0">
                              <Radio className={`w-4 h-4 flex-shrink-0 ${isAlive ? 'text-emerald-400' : 'text-amber-400'}`} />
                              <div className="min-w-0">
                                <div className="text-sm font-medium text-slate-200 truncate flex items-center gap-1.5">
                                  <span>{node.name}</span>
                                  <span className={`text-[10px] px-1.5 py-0.2 rounded font-mono border ${
                                    isAlive 
                                      ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' 
                                      : 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                                  }`}>
                                    {node.status}
                                  </span>
                                </div>
                                <div className="text-xs text-slate-400 truncate flex items-center gap-1">
                                  <Globe className="w-3 h-3" />
                                  <span>{node.addresses[0] || 'no address'}</span>
                                </div>
                              </div>
                            </div>
                            {isSelected && <Check className="w-4 h-4 text-rose-400 flex-shrink-0" />}
                          </button>
                        );
                      })}

                    {nodes.filter(n => !n.is_self).length === 0 && (
                      <div className="px-3.5 py-3 text-center text-xs text-slate-500">
                        No remote peer nodes discovered yet
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* Global Refresh Button */}
            <button
              onClick={onRefresh}
              disabled={isLoading}
              className="p-2 rounded-lg bg-slate-800/80 hover:bg-slate-800 text-slate-300 hover:text-white border border-slate-700/80 transition-all disabled:opacity-50"
              title="Refresh all data"
            >
              <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin text-rose-400' : ''}`} />
            </button>
          </div>
        </div>
      </div>
    </header>
  );
};
