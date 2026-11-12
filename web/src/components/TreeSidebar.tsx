import React, { useState, useMemo, useEffect } from 'react';
import { Link, useRouterState } from '@tanstack/react-router';
import { resourceTheme } from '../theme';
import {
  ChevronRight,
  ChevronDown,
  Layers,
  Box,
  Server,
  Network,
  HardDrive,
  Search,
  X,
} from 'lucide-react';
import { Node } from '../types';
import {
  useNodesQuery,
  useClusterStacksQuery,
  useClusterContainersQuery,
} from '../hooks/useClusterData';
import { Input } from './ui/input';
import { Badge } from './ui/badge';
import { ScrollArea } from './ui/scroll-area';
import { NodeHealthDot } from './NodeHealthDot';

interface TreeSidebarProps {
  onItemClick?: () => void;
}

export const TreeSidebar: React.FC<TreeSidebarProps> = ({ onItemClick }) => {
  const routerState = useRouterState();
  const pathname = routerState.location.pathname;

  const [filterText, setFilterText] = useState('');
  const [expandedNodes, setExpandedNodes] = useState<Record<string, boolean>>({});

  const updateExpanded = (key: string, value: boolean) => {
    setExpandedNodes((prev) => ({ ...prev, [key]: value }));
  };

  const toggleExpand = (key: string, defaultExpanded: boolean = false) => {
    const isCurrent = expandedNodes[key] !== undefined ? expandedNodes[key] : defaultExpanded;
    updateExpanded(key, !isCurrent);
  };

  // Fetch cluster data
  const { data: nodes = [] } = useNodesQuery();
  const { data: stacksData } = useClusterStacksQuery();
  const { data: containersData } = useClusterContainersQuery();

  const stacks = stacksData?.items ?? [];
  const containers = containersData?.items ?? [];

  // Group stacks & containers by node
  const nodeDataMap = useMemo(() => {
    const map = new Map<
      string,
      {
        node: Node;
        stacks: typeof stacks;
        containers: typeof containers;
        stackContainers: Map<string, typeof containers>;
        standaloneContainers: typeof containers;
      }
    >();

    nodes.forEach((n) => {
      const nodeStacks = stacks.filter((s) => s.hostId === n.id);
      const nodeContainers = containers.filter((c) => c.hostId === n.id);

      const stackContainers = new Map<string, typeof containers>();
      const standaloneContainers: typeof containers = [];

      nodeContainers.forEach((c) => {
        if (c.stack) {
          const list = stackContainers.get(c.stack) || [];
          list.push(c);
          stackContainers.set(c.stack, list);
        } else {
          standaloneContainers.push(c);
        }
      });

      map.set(n.id, {
        node: n,
        stacks: nodeStacks,
        containers: nodeContainers,
        stackContainers,
        standaloneContainers,
      });
    });

    return map;
  }, [nodes, stacks, containers]);

  const query = filterText.trim().toLowerCase();

  // Auto-reveal on route change: ensure parent tree branches are expanded and scroll item into view
  useEffect(() => {
    // Check if route is a node-scoped route: /nodes/:nodeId/...
    const match = pathname.match(/^\/nodes\/([^/]+)(?:\/(stacks|containers)(?:\/([^/]+))?)?/);
    if (!match) return;

    const [, nodeId, resourceType, resourceId] = match;
    const updates: Record<string, boolean> = {};

    // 1. If at node root (/nodes/:nodeId), node itself is the target; do not expand its children.
    // Only expand ancestor branches needed to reveal the current item:
    if (!resourceType) {
      // At node level: node is visible under NODES section. Nothing to expand.
    } else {
      // Child of node: expand node to reveal stacks/containers
      updates[`node_${nodeId}`] = true;

      if (resourceType === 'stacks') {
        if (resourceId) {
          // Viewing a specific stack (/nodes/:nodeId/stacks/:stackName):
          // Expand the parent 'Stacks' folder so this stack is visible.
          // Do NOT expand the stack itself (its child containers stay collapsed).
          updates[`node_${nodeId}_stacks`] = true;
        }
      } else if (resourceType === 'containers') {
        if (resourceId) {
          // Viewing a specific container (/nodes/:nodeId/containers/:containerId):
          // Find if it's inside a stack or standalone, and expand parent(s) to reveal it.
          const nodeData = nodeDataMap.get(nodeId);
          const containerObj = nodeData?.containers.find((c) => c.id === resourceId);
          if (containerObj?.stack) {
            updates[`node_${nodeId}_stacks`] = true;
            updates[`node_${nodeId}_stack_${containerObj.stack}`] = true;
          } else if (containerObj) {
            updates[`node_${nodeId}_standalone`] = true;
          }
        }
      }
    }

    setExpandedNodes((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const [k, v] of Object.entries(updates)) {
        if (!next[k]) {
          next[k] = v;
          changed = true;
        }
      }
      return changed ? next : prev;
    });

    // Scroll active item into view within the sidebar
    const timer = setTimeout(() => {
      const activeEl = document.querySelector('[data-sidebar-active="true"]');
      if (activeEl) {
        activeEl.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
      }
    }, 100);

    return () => clearTimeout(timer);
  }, [pathname, nodeDataMap]);

  // Helper to determine active link
  const isExactActive = (target: string) => pathname === target;

  return (
    <aside className="w-64 flex flex-col h-full bg-slate-950/90 border-r border-slate-800 select-none text-xs">
      {/* Search / Filter Header */}
      <div className="p-3 border-b border-slate-850">
        <div className="relative">
          <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
          <Input
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            placeholder="Quick filter..."
            className="h-8 pl-8 pr-7 text-xs bg-slate-900 border-slate-800 w-full focus:ring-rose-500 text-slate-200 placeholder:text-slate-500 rounded-md"
          />
          {filterText && (
            <button
              onClick={() => setFilterText('')}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-white p-0.5"
              aria-label="Clear filter"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>

      <ScrollArea className="flex-1 py-2 px-2">
        <div className="space-y-4">
          {/* SECTION: CLUSTER VIEWS */}
          <div>
            <div className="px-2 pb-1 text-[10px] font-semibold tracking-wider text-slate-400 uppercase">
              Cluster
            </div>
            <div className="space-y-0.5">
              {/* All Stacks */}
              {(!query || 'all stacks'.includes(query) || 'stacks'.includes(query)) && (
                <Link
                  to="/stacks"
                  onClick={onItemClick}
                  data-sidebar-active={isExactActive('/stacks')}
                  className={`group flex items-center justify-between px-2.5 py-1.5 rounded-md transition-colors ${
                    isExactActive('/stacks')
                      ? resourceTheme.stack.activeBg
                      : 'text-slate-300 hover:bg-slate-900 hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <Layers className={`w-3.5 h-3.5 flex-shrink-0 ${resourceTheme.stack.icon}`} />
                    <span className="truncate">All Stacks</span>
                  </div>
                  <span className="text-[10px] text-slate-400 font-mono">
                    {stacks.length}
                  </span>
                </Link>
              )}

              {/* All Containers */}
              {(!query || 'all containers'.includes(query) || 'containers'.includes(query)) && (
                <Link
                  to="/containers"
                  onClick={onItemClick}
                  data-sidebar-active={isExactActive('/containers')}
                  className={`group flex items-center justify-between px-2.5 py-1.5 rounded-md transition-colors ${
                    isExactActive('/containers')
                      ? resourceTheme.container.activeBg
                      : 'text-slate-300 hover:bg-slate-900 hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <Box className={`w-3.5 h-3.5 flex-shrink-0 ${resourceTheme.container.icon}`} />
                    <span className="truncate">All Containers</span>
                  </div>
                  <span className="text-[10px] text-slate-400 font-mono">
                    {containers.length}
                  </span>
                </Link>
              )}

              {/* All Nodes */}
              {(!query || 'all nodes'.includes(query) || 'nodes'.includes(query)) && (
                <Link
                  to="/nodes"
                  onClick={onItemClick}
                  data-sidebar-active={isExactActive('/nodes')}
                  className={`group flex items-center justify-between px-2.5 py-1.5 rounded-md transition-colors ${
                    isExactActive('/nodes')
                      ? resourceTheme.node.activeBg
                      : 'text-slate-300 hover:bg-slate-900 hover:text-white'
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <Server className={`w-3.5 h-3.5 flex-shrink-0 ${resourceTheme.node.icon}`} />
                    <span className="truncate">All Nodes</span>
                  </div>
                  <span className="text-[10px] text-slate-400 font-mono">
                    {nodes.length}
                  </span>
                </Link>
              )}

              {/* Placeholders for Future Networks & Volumes */}
              {!query && (
                <>
                  <div
                    className="flex items-center justify-between px-2.5 py-1.5 rounded-md text-slate-500 cursor-not-allowed opacity-50"
                    title="Networks (Coming soon)"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <Network className="w-3.5 h-3.5 flex-shrink-0" />
                      <span className="truncate">All Networks</span>
                    </div>
                    <Badge variant="outline" className="text-[9px] px-1 py-0 border-slate-800 text-slate-500">
                      soon
                    </Badge>
                  </div>
                  <div
                    className="flex items-center justify-between px-2.5 py-1.5 rounded-md text-slate-500 cursor-not-allowed opacity-50"
                    title="Volumes (Coming soon)"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <HardDrive className="w-3.5 h-3.5 flex-shrink-0" />
                      <span className="truncate">All Volumes</span>
                    </div>
                    <Badge variant="outline" className="text-[9px] px-1 py-0 border-slate-800 text-slate-500">
                      soon
                    </Badge>
                  </div>
                </>
              )}
            </div>
          </div>

          {/* SECTION: NODES DRILL-DOWN */}
          <div>
            <div className="px-2 pb-1 text-[10px] font-semibold tracking-wider text-slate-400 uppercase">
              Nodes
            </div>

            {nodes.length === 0 ? (
              <div className="px-2.5 py-2 text-slate-500 italic text-[11px]">
                No nodes available
              </div>
            ) : (
              <div className="space-y-1">
                {nodes.map((node) => {
                  const nodeEntry = nodeDataMap.get(node.id);
                  const nodeStacks = nodeEntry?.stacks ?? [];
                  const nodeContainers = nodeEntry?.containers ?? [];
                  const stackContainers = nodeEntry?.stackContainers;
                  const standalone = nodeEntry?.standaloneContainers ?? [];

                  // Check filter match
                  const nodeMatch = !query || node.name.toLowerCase().includes(query) || node.id.includes(query);
                  const anyStackMatch = nodeStacks.some((s) => s.name.toLowerCase().includes(query));
                  const anyContainerMatch = nodeContainers.some((c) =>
                    c.name.toLowerCase().includes(query) || c.image.toLowerCase().includes(query)
                  );

                  if (query && !nodeMatch && !anyStackMatch && !anyContainerMatch) {
                    return null;
                  }

                  // Default expanded if currently on this node's route or if user is searching
                  const isCurrentNodeRoute = pathname.startsWith(`/nodes/${node.id}`);
                  const nodeExpandKey = `node_${node.id}`;
                  const isNodeExpanded =
                    query !== ''
                      ? true
                      : (expandedNodes[nodeExpandKey] ?? isCurrentNodeRoute);

                  return (
                    <div key={node.id} className="space-y-0.5">
                      {/* Node Header Row */}
                      <div
                        data-sidebar-active={isExactActive(`/nodes/${node.id}`)}
                        className={`group flex items-center justify-between px-1.5 py-1 rounded-md transition-colors ${
                          isExactActive(`/nodes/${node.id}`)
                            ? resourceTheme.node.activeBg
                            : 'text-slate-300 hover:bg-slate-900 hover:text-white'
                        }`}
                      >
                        <div className="flex items-center gap-1.5 min-w-0 flex-1">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.preventDefault();
                              e.stopPropagation();
                              toggleExpand(nodeExpandKey, isCurrentNodeRoute);
                            }}
                            className="p-0.5 text-slate-400 hover:text-white rounded"
                            aria-label="Toggle node"
                          >
                            {isNodeExpanded ? (
                              <ChevronDown className="w-3.5 h-3.5 text-slate-400" />
                            ) : (
                              <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
                            )}
                          </button>

                          <Link
                            to="/nodes/$nodeId"
                            params={{ nodeId: node.id }}
                            onClick={onItemClick}
                            className="flex items-center gap-1.5 min-w-0 flex-1"
                          >
                            <Server className={`w-3.5 h-3.5 flex-shrink-0 ${resourceTheme.node.icon}`} />
                            <span className="truncate font-medium">{node.name}</span>
                          </Link>
                        </div>

                        <div className="flex items-center gap-1.5 pl-1 flex-shrink-0">
                          {node.is_self && (
                            <span className="text-[9px] px-1 py-0 rounded bg-slate-800 text-slate-400 font-mono border border-slate-700">
                              self
                            </span>
                          )}
                          <NodeHealthDot status={node.status} />
                        </div>
                      </div>

                      {/* Sub-tree for Node */}
                      {isNodeExpanded && (
                        <div className="pl-4 ml-2 border-l border-slate-800/80 space-y-0.5 mt-0.5">
                          {/* 1. Stacks branch */}
                          {(() => {
                            const stacksKey = `node_${node.id}_stacks`;
                            const isStacksRoute =
                              pathname === `/nodes/${node.id}/stacks` ||
                              pathname.startsWith(`/nodes/${node.id}/stacks/`);
                            const isStacksExpanded =
                              query !== ''
                                ? true
                                : (expandedNodes[stacksKey] ?? (isStacksRoute || nodeStacks.length > 0));

                            return (
                              <div className="space-y-0.5">
                                <div
                                  data-sidebar-active={isExactActive(`/nodes/${node.id}/stacks`)}
                                  className={`group flex items-center justify-between px-1.5 py-1 rounded-md transition-colors ${
                                    isExactActive(`/nodes/${node.id}/stacks`)
                                      ? resourceTheme.stack.activeBg
                                      : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'
                                  }`}
                                >
                                  <div className="flex items-center gap-1 min-w-0 flex-1">
                                    {nodeStacks.length > 0 ? (
                                      <button
                                        type="button"
                                        onClick={(e) => {
                                          e.preventDefault();
                                          e.stopPropagation();
                                          toggleExpand(stacksKey, true);
                                        }}
                                        className="p-0.5 text-slate-400 hover:text-white rounded"
                                      >
                                        {isStacksExpanded ? (
                                          <ChevronDown className="w-3 h-3 text-slate-400" />
                                        ) : (
                                          <ChevronRight className="w-3 h-3 text-slate-400" />
                                        )}
                                      </button>
                                    ) : (
                                      <span className="w-4" />
                                    )}
                                    <Link
                                      to="/nodes/$nodeId/stacks"
                                      params={{ nodeId: node.id }}
                                      onClick={onItemClick}
                                      className="flex items-center gap-1.5 min-w-0 flex-1"
                                    >
                                      <Layers className={`w-3.5 h-3.5 flex-shrink-0 ${resourceTheme.stack.icon}`} />
                                      <span className="truncate">Stacks</span>
                                    </Link>
                                  </div>
                                  <span className="text-[10px] text-slate-400 font-mono">
                                    {nodeStacks.length}
                                  </span>
                                </div>

                                {/* Stack Items */}
                                {isStacksExpanded && nodeStacks.length > 0 && (
                                  <div className="pl-4 ml-1.5 border-l border-slate-800/60 space-y-0.5">
                                    {nodeStacks.map((stk) => {
                                      const stkMatch = !query || stk.name.toLowerCase().includes(query);
                                      const stkContainers = stackContainers?.get(stk.name) ?? [];
                                      const anyStkContainerMatch = stkContainers.some((c) =>
                                        c.name.toLowerCase().includes(query)
                                      );

                                      if (query && !stkMatch && !anyStkContainerMatch) {
                                        return null;
                                      }

                                      const stackKey = `node_${node.id}_stack_${stk.name}`;
                                      const isStkRoute = pathname === `/nodes/${node.id}/stacks/${stk.name}`;
                                      const isStkExpanded =
                                        query !== ''
                                          ? true
                                          : (expandedNodes[stackKey] ?? isStkRoute);

                                      return (
                                        <div key={stk.name} className="space-y-0.5">
                                          <div
                                            data-sidebar-active={isStkRoute}
                                            className={`group flex items-center justify-between px-1.5 py-1 rounded-md transition-colors ${
                                              isStkRoute
                                                ? resourceTheme.stack.activeBg
                                                : 'text-slate-300 hover:bg-slate-900 hover:text-white'
                                            }`}
                                          >
                                            <div className="flex items-center gap-1 min-w-0 flex-1">
                                              {stkContainers.length > 0 ? (
                                                <button
                                                  type="button"
                                                  onClick={(e) => {
                                                    e.preventDefault();
                                                    e.stopPropagation();
                                                    toggleExpand(stackKey, false);
                                                  }}
                                                  className="p-0.5 text-slate-400 hover:text-white rounded"
                                                >
                                                  {isStkExpanded ? (
                                                    <ChevronDown className="w-3 h-3 text-slate-400" />
                                                  ) : (
                                                    <ChevronRight className="w-3 h-3 text-slate-400" />
                                                  )}
                                                </button>
                                              ) : (
                                                <span className="w-4" />
                                              )}
                                              <Link
                                                to="/nodes/$nodeId/stacks/$stackName"
                                                params={{ nodeId: node.id, stackName: stk.name }}
                                                onClick={onItemClick}
                                                className="truncate flex-1 font-mono text-[11px]"
                                                title={stk.name}
                                              >
                                                {stk.name}
                                              </Link>
                                            </div>
                                            <span className="text-[10px] text-slate-400 font-mono">
                                              {stk.rollup?.running ?? 0}/{stk.services?.length ?? 0}
                                            </span>
                                          </div>

                                          {/* Containers inside this stack */}
                                          {isStkExpanded && stkContainers.length > 0 && (
                                            <div className="pl-4 ml-1.5 border-l border-slate-800/40 space-y-0.5">
                                              {stkContainers.map((c) => {
                                                const cleanName = c.name.replace(/^\//, '');
                                                if (query && !cleanName.toLowerCase().includes(query)) {
                                                  return null;
                                                }
                                                const isContainerActive =
                                                  pathname === `/nodes/${node.id}/containers/${c.id}`;
                                                const isRunning = c.state === 'running';

                                                return (
                                                  <Link
                                                    key={c.id}
                                                    to="/nodes/$nodeId/containers/$containerId"
                                                    params={{ nodeId: node.id, containerId: c.id }}
                                                    onClick={onItemClick}
                                                    data-sidebar-active={isContainerActive}
                                                    className={`group flex items-center justify-between px-2 py-0.5 rounded transition-colors text-[11px] font-mono ${
                                                      isContainerActive
                                                        ? resourceTheme.container.activeBg
                                                        : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'
                                                    }`}
                                                    title={cleanName}
                                                  >
                                                    <div className="flex items-center gap-1.5 min-w-0">
                                                      <span
                                                        className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
                                                          isRunning ? 'bg-emerald-400' : 'bg-slate-600'
                                                        }`}
                                                      />
                                                      <span className="truncate">{cleanName}</span>
                                                    </div>
                                                  </Link>
                                                );
                                              })}
                                            </div>
                                          )}
                                        </div>
                                      );
                                    })}
                                  </div>
                                )}
                              </div>
                            );
                          })()}

                          {/* 2. All Containers on Node */}
                          {(() => {
                            const isContainersRoute = pathname === `/nodes/${node.id}/containers`;
                            return (
                              <Link
                                to="/nodes/$nodeId/containers"
                                params={{ nodeId: node.id }}
                                onClick={onItemClick}
                                className={`group flex items-center justify-between px-1.5 py-1 rounded-md transition-colors ${
                                  isContainersRoute
                                    ? resourceTheme.container.activeBg
                                    : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'
                                }`}
                              >
                                <div className="flex items-center gap-1.5 min-w-0">
                                  <span className="w-4" />
                                  <Box className={`w-3.5 h-3.5 flex-shrink-0 ${resourceTheme.container.icon}`} />
                                  <span className="truncate">Containers</span>
                                </div>
                                <span className="text-[10px] text-slate-400 font-mono">
                                  {nodeContainers.length}
                                </span>
                              </Link>
                            );
                          })()}

                          {/* 3. Standalone Containers Group (if any exist) */}
                          {standalone.length > 0 && (!query || standalone.some((c) => c.name.toLowerCase().includes(query))) && (
                            (() => {
                              const standaloneKey = `node_${node.id}_standalone`;
                              const isStandaloneExpanded =
                                query !== '' ? true : (expandedNodes[standaloneKey] ?? false);

                              return (
                                <div className="space-y-0.5">
                                  <div
                                    className="group flex items-center justify-between px-1.5 py-1 rounded-md text-slate-400 hover:bg-slate-900 hover:text-slate-200 transition-colors cursor-pointer"
                                    onClick={() => toggleExpand(standaloneKey, false)}
                                  >
                                    <div className="flex items-center gap-1 min-w-0 flex-1">
                                      <button
                                        type="button"
                                        className="p-0.5 text-slate-400 hover:text-white rounded"
                                      >
                                        {isStandaloneExpanded ? (
                                          <ChevronDown className="w-3 h-3 text-slate-400" />
                                        ) : (
                                          <ChevronRight className="w-3 h-3 text-slate-400" />
                                        )}
                                      </button>
                                      <span className="truncate text-slate-400">Standalone</span>
                                    </div>
                                    <span className="text-[10px] text-slate-400 font-mono">
                                      {standalone.length}
                                    </span>
                                  </div>

                                  {isStandaloneExpanded && (
                                    <div className="pl-4 ml-1.5 border-l border-slate-800/40 space-y-0.5">
                                      {standalone.map((c) => {
                                        const cleanName = c.name.replace(/^\//, '');
                                        if (query && !cleanName.toLowerCase().includes(query)) {
                                          return null;
                                        }
                                        const isContainerActive =
                                          pathname === `/nodes/${node.id}/containers/${c.id}`;
                                        const isRunning = c.state === 'running';

                                        return (
                                          <Link
                                            key={c.id}
                                            to="/nodes/$nodeId/containers/$containerId"
                                            params={{ nodeId: node.id, containerId: c.id }}
                                            onClick={onItemClick}
                                            data-sidebar-active={isContainerActive}
                                            className={`group flex items-center justify-between px-2 py-0.5 rounded transition-colors text-[11px] font-mono ${
                                              isContainerActive
                                                ? resourceTheme.container.activeBg
                                                : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'
                                            }`}
                                            title={cleanName}
                                          >
                                            <div className="flex items-center gap-1.5 min-w-0">
                                              <span
                                                className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
                                                  isRunning ? 'bg-emerald-400' : 'bg-slate-600'
                                                }`}
                                              />
                                              <span className="truncate">{cleanName}</span>
                                            </div>
                                          </Link>
                                        );
                                      })}
                                    </div>
                                  )}
                                </div>
                              );
                            })()
                          )}

                          {/* Placeholders for Future Node Networks & Volumes */}
                          {!query && (
                            <>
                              <div
                                className="flex items-center justify-between px-1.5 py-1 text-slate-500 opacity-50 cursor-not-allowed text-[11px]"
                                title="Networks (Coming soon)"
                              >
                                <div className="flex items-center gap-1.5 min-w-0">
                                  <span className="w-4" />
                                  <Network className="w-3 h-3 flex-shrink-0" />
                                  <span className="truncate">Networks</span>
                                </div>
                                <span className="text-[9px] font-mono">soon</span>
                              </div>
                              <div
                                className="flex items-center justify-between px-1.5 py-1 text-slate-500 opacity-50 cursor-not-allowed text-[11px]"
                                title="Volumes (Coming soon)"
                              >
                                <div className="flex items-center gap-1.5 min-w-0">
                                  <span className="w-4" />
                                  <HardDrive className="w-3 h-3 flex-shrink-0" />
                                  <span className="truncate">Volumes</span>
                                </div>
                                <span className="text-[9px] font-mono">soon</span>
                              </div>
                            </>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </ScrollArea>
    </aside>
  );
};
