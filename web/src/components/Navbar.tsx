import React from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import { useNodesQuery } from '../hooks/useClusterData';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from './ui/dropdown-menu';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Badge } from './ui/badge';
import {
  Activity,
  Layers,
  Box,
  Server,
  ChevronDown,
  Check,
  RefreshCw,
  Search,
  Globe,
  Radio,
} from 'lucide-react';

interface NavbarProps {
  selectedHostId: string | null;
  onSelectHostId: (hostId: string | null) => void;
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onRefresh: () => void;
  isRefreshing?: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  selectedHostId,
  onSelectHostId,
  searchQuery,
  onSearchChange,
  onRefresh,
  isRefreshing = false,
}) => {
  const { data: nodes = [] } = useNodesQuery();
  const routerState = useRouterState();
  const pathname = routerState.location.pathname;

  const selectedNode = nodes.find((n) => n.id === selectedHostId);
  const hostLabel = selectedNode ? selectedNode.name : 'All Hosts';

  return (
    <header className="sticky top-0 z-40 w-full border-b border-slate-800/80 bg-slate-950/80 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 flex items-center justify-between h-14 gap-4">
        {/* Left: Brand + Navigation links */}
        <div className="flex items-center gap-6">
          <Link to="/stacks" className="flex items-center gap-2.5 group">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-tr from-rose-600 via-pink-500 to-rose-400 flex items-center justify-center shadow-md shadow-rose-950/50 group-hover:scale-105 transition-transform">
              <Activity className="w-4 h-4 text-white" />
            </div>
            <span className="font-bold text-base tracking-tight text-white group-hover:text-rose-400 transition-colors">
              Dokidoki
            </span>
          </Link>

          {/* Nav links */}
          <nav className="flex items-center gap-1">
            <Link
              to="/stacks"
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                pathname.startsWith('/stacks') || pathname === '/'
                  ? 'bg-slate-800 text-white font-semibold shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <Layers className="w-3.5 h-3.5 text-rose-500" />
              Stacks
            </Link>

            <Link
              to="/containers"
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                pathname.startsWith('/containers')
                  ? 'bg-slate-800 text-white font-semibold shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <Box className="w-3.5 h-3.5 text-blue-400" />
              Containers
            </Link>

            <Link
              to="/nodes"
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                pathname.startsWith('/nodes')
                  ? 'bg-slate-800 text-white font-semibold shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <Server className="w-3.5 h-3.5 text-emerald-400" />
              Nodes
            </Link>
          </nav>
        </div>

        {/* Right Controls: Host filter, Search, Refresh */}
        <div className="flex items-center gap-2.5">
          {/* Host Filter Selector */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-850 border border-slate-800 text-slate-200 text-xs font-medium transition-colors focus:outline-none focus:ring-1 focus:ring-rose-500"
              >
                <Server className="w-3.5 h-3.5 text-slate-400" />
                <span className="max-w-[120px] sm:max-w-[150px] truncate">{hostLabel}</span>
                <ChevronDown className="w-3 h-3 text-slate-500" />
              </button>
            </DropdownMenuTrigger>

            <DropdownMenuContent align="end" className="w-64 bg-slate-900 border-slate-800 text-slate-200">
              <DropdownMenuLabel className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
                Cluster Nodes
              </DropdownMenuLabel>

              <DropdownMenuItem
                onClick={() => onSelectHostId(null)}
                className="cursor-pointer flex items-center justify-between text-xs py-2 hover:bg-slate-800"
              >
                <div className="flex items-center gap-2">
                  <Globe className="w-3.5 h-3.5 text-slate-400" />
                  <div>
                    <div className="font-semibold text-slate-100">All Hosts</div>
                    <div className="text-[10px] text-slate-500">All cluster nodes</div>
                  </div>
                </div>
                {!selectedHostId && <Check className="w-4 h-4 text-rose-400" />}
              </DropdownMenuItem>

              <DropdownMenuSeparator className="bg-slate-800" />

              {nodes.map((node) => {
                const isSelected = selectedHostId === node.id;
                const isAlive = node.status === 'alive';

                return (
                  <DropdownMenuItem
                    key={node.id}
                    onClick={() => onSelectHostId(node.id)}
                    className="cursor-pointer flex items-center justify-between text-xs py-2 hover:bg-slate-800"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <Radio className={`w-3.5 h-3.5 flex-shrink-0 ${isAlive ? 'text-emerald-400' : 'text-amber-400'}`} />
                      <div className="min-w-0">
                        <div className="font-semibold text-slate-100 truncate flex items-center gap-1.5">
                          <span>{node.name}</span>
                          {node.is_self && (
                            <Badge variant="outline" className="text-[9px] px-1 py-0 bg-slate-800 text-slate-400 border-slate-700">
                              Origin
                            </Badge>
                          )}
                        </div>
                        <div className="text-[10px] text-slate-500 font-mono truncate">
                          {node.addresses?.[0] || 'direct socket'}
                        </div>
                      </div>
                    </div>
                    {isSelected && <Check className="w-4 h-4 text-rose-400 flex-shrink-0" />}
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Quick Search Input */}
          <div className="relative hidden md:block">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <Input
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder="Search cluster..."
              className="h-8 pl-8 pr-2.5 text-xs bg-slate-900 border-slate-800 w-44 lg:w-56 focus:ring-rose-500 text-slate-200"
            />
          </div>

          {/* Global Refresh Button */}
          <Button
            size="sm"
            variant="ghost"
            onClick={onRefresh}
            disabled={isRefreshing}
            className="h-8 w-8 p-0 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-white"
            title="Refresh data"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin text-rose-500' : ''}`} />
          </Button>
        </div>
      </div>
    </header>
  );
};
