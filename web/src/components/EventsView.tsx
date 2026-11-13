import React, { useState, useEffect, useRef } from 'react';
import { DockerEvent } from '../types';
import { api } from '../services/api';
import { useNodesQuery, getNodeEndpoint } from '../hooks/useClusterData';
import { Badge } from './ui/badge';
import { Input } from './ui/input';
import { Button } from './ui/button';
import {
  Activity,
  Server,
  Search,
  Pause,
  Play,
  Trash2,
  X,
  Radio,
  Box,
  HardDrive,
  Layers,
  Network,
} from 'lucide-react';

interface EventsViewProps {
  selectedHostId?: string | null;
}

export const EventsView: React.FC<EventsViewProps> = ({ selectedHostId }) => {
  const [events, setEvents] = useState<DockerEvent[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [, setConnectedNodes] = useState<Record<string, boolean>>({});

  const { data: nodes = [] } = useNodesQuery();
  const isPausedRef = useRef(isPaused);
  isPausedRef.current = isPaused;

  const targetNodes = React.useMemo(() => {
    if (!selectedHostId || selectedHostId === 'all') {
      return nodes.filter((n) => n.status !== 'offline');
    }
    return nodes.filter((n) => n.id === selectedHostId);
  }, [nodes, selectedHostId]);

  useEffect(() => {
    const activeNodes = targetNodes.length > 0
      ? targetNodes
      : [{ id: 'local', name: 'Local Host', addresses: [], status: 'alive' as const, version: '', is_self: true }];

    const eventSources: EventSource[] = [];

    activeNodes.forEach((node) => {
      const endpoint = getNodeEndpoint(node);
      try {
        const es = api.getEventsEventSource(endpoint);

        es.onopen = () => {
          setConnectedNodes((prev) => ({ ...prev, [node.id]: true }));
        };

        es.onerror = () => {
          setConnectedNodes((prev) => ({ ...prev, [node.id]: false }));
        };

        es.onmessage = (msg) => {
          if (isPausedRef.current) return;
          try {
            const data: DockerEvent = JSON.parse(msg.data);
            data.hostId = node.id;
            data.hostName = node.name;
            setEvents((prev) => [data, ...prev.slice(0, 499)]); // keep up to 500 events
          } catch (e) {
            console.error('Error parsing docker event:', e);
          }
        };

        eventSources.push(es);
      } catch (err) {
        console.error('Failed to create EventSource for node:', node.name, err);
      }
    });

    return () => {
      eventSources.forEach((es) => es.close());
    };
  }, [targetNodes]);

  const eventTypes = React.useMemo(() => {
    const types = new Set<string>();
    events.forEach((e) => {
      if (e.Type) types.add(e.Type);
    });
    return Array.from(types).sort();
  }, [events]);

  const filteredEvents = React.useMemo(() => {
    const q = search.trim().toLowerCase();
    return events.filter((e) => {
      if (typeFilter !== 'all' && e.Type !== typeFilter) return false;
      if (q) {
        const actorId = e.Actor?.ID || (e.Actor as any)?.Id || '';
        const matchType = (e.Type || '').toLowerCase().includes(q);
        const matchAction = (e.Action || '').toLowerCase().includes(q);
        const matchActor = actorId.toLowerCase().includes(q);
        const matchHost = (e.hostName || '').toLowerCase().includes(q);
        const matchAttrs = Object.entries(e.Actor?.Attributes || {}).some(
          ([k, v]) => k.toLowerCase().includes(q) || (v || '').toLowerCase().includes(q)
        );
        if (!matchType && !matchAction && !matchActor && !matchHost && !matchAttrs) {
          return false;
        }
      }
      return true;
    });
  }, [events, typeFilter, search]);

  const getTypeIcon = (type: string) => {
    switch (type.toLowerCase()) {
      case 'container':
        return <Box className="w-3.5 h-3.5 text-slate-400" />;
      case 'network':
        return <Network className="w-3.5 h-3.5 text-slate-400" />;
      case 'volume':
        return <HardDrive className="w-3.5 h-3.5 text-slate-400" />;
      case 'image':
        return <Layers className="w-3.5 h-3.5 text-slate-400" />;
      case 'node':
      case 'daemon':
        return <Server className="w-3.5 h-3.5 text-slate-400" />;
      default:
        return <Activity className="w-3.5 h-3.5 text-slate-400" />;
    }
  };

  const getActionBadgeClass = (action: string) => {
    const act = action.toLowerCase();
    if (act.includes('start') || act.includes('create') || act.includes('connect')) {
      return 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60';
    }
    if (act.includes('die') || act.includes('kill') || act.includes('destroy') || act.includes('stop')) {
      return 'bg-rose-950/40 text-rose-400 border-rose-800/60';
    }
    if (act.includes('pause') || act.includes('restart') || act.includes('disconnect')) {
      return 'bg-amber-950/40 text-amber-400 border-amber-800/60';
    }
    return 'bg-slate-900 border-slate-700 text-slate-300';
  };

  return (
    <div className="space-y-4">
      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Activity className="w-5 h-5 text-slate-400" />
            <h2 className="text-lg font-bold text-slate-100">Live Docker Events</h2>
          </div>
          <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-slate-900 border border-slate-800 text-[11px] text-slate-300">
            <Radio className={`w-3 h-3 ${isPaused ? 'text-amber-400' : 'text-emerald-400 animate-pulse'}`} />
            <span>{isPaused ? 'Paused' : 'Streaming'}</span>
          </div>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {filteredEvents.length} events
          </Badge>
        </div>

        {/* Action Controls & Search */}
        <div className="flex items-center gap-2">
          <div className="relative w-full sm:w-56">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search events..."
              className="pl-9 pr-8 h-8 text-xs bg-slate-900/80 border-slate-700 text-slate-100 placeholder:text-slate-400"
            />
            {search && (
              <button
                onClick={() => setSearch('')}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsPaused(!isPaused)}
            className="h-8 px-2.5 border-slate-800 bg-slate-900 hover:bg-slate-800 text-slate-300"
          >
            {isPaused ? (
              <>
                <Play className="w-3.5 h-3.5 mr-1 text-emerald-400" />
                Resume
              </>
            ) : (
              <>
                <Pause className="w-3.5 h-3.5 mr-1 text-amber-400" />
                Pause
              </>
            )}
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setEvents([])}
            className="h-8 px-2.5 border-slate-800 bg-slate-900 hover:bg-slate-800 text-slate-400 hover:text-rose-400"
            title="Clear event log"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </div>

      {/* Type filter pills */}
      {eventTypes.length > 0 && (
        <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs">
          <span className="text-slate-400 font-medium mr-1">Type:</span>
          <button
            onClick={() => setTypeFilter('all')}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
              typeFilter === 'all'
                ? 'bg-slate-800 text-white border border-slate-700'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
            }`}
          >
            All
          </button>
          {eventTypes.map((t) => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors capitalize ${
                typeFilter === t
                  ? 'bg-slate-800 text-white border border-slate-700'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
              }`}
            >
              {t}
            </button>
          ))}
        </div>
      )}

      {/* Events table / list */}
      {filteredEvents.length === 0 ? (
        <div className="text-center py-20 border border-dashed border-slate-800 rounded-xl p-8">
          <Activity className="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">Listening for Docker events...</h3>
          <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
            Events will appear here in real-time as containers, networks, volumes, and images change state.
          </p>
        </div>
      ) : (
        <div className="border border-slate-800/80 rounded-xl overflow-hidden bg-slate-900/20">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="border-b border-slate-800 bg-slate-900/60 text-slate-400 font-medium">
                <th className="py-2.5 px-4">Time</th>
                <th className="py-2.5 px-3">Type</th>
                <th className="py-2.5 px-3">Action</th>
                <th className="py-2.5 px-3">Actor / Resource</th>
                <th className="py-2.5 px-3">Attributes</th>
                <th className="py-2.5 px-4 text-right">Host Node</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono">
              {filteredEvents.map((ev, idx) => {
                const date = new Date(ev.Time * 1000);
                const timeStr = date.toLocaleTimeString();
                const actorId = ev.Actor?.ID || (ev.Actor as any)?.Id || '';
                const actorName = ev.Actor?.Attributes?.name || (actorId ? actorId.substring(0, 12) : '-');

                return (
                  <tr key={`${ev.TimeNano}-${idx}`} className="hover:bg-slate-800/40 transition-colors">
                    <td className="py-2 px-4 text-slate-400 tabular-nums whitespace-nowrap">
                      {timeStr}
                    </td>
                    <td className="py-2 px-3 whitespace-nowrap font-sans">
                      <div className="flex items-center gap-1.5">
                        {getTypeIcon(ev.Type)}
                        <span className="capitalize text-slate-300">{ev.Type}</span>
                      </div>
                    </td>
                    <td className="py-2 px-3 whitespace-nowrap">
                      <Badge variant="outline" className={`text-[10px] uppercase font-semibold ${getActionBadgeClass(ev.Action)}`}>
                        {ev.Action}
                      </Badge>
                    </td>
                    <td className="py-2 px-3 text-slate-200 truncate max-w-xs font-sans">
                      <div className="truncate font-medium">{actorName}</div>
                      {actorId && (
                        <div className="text-[10px] text-slate-500 font-mono truncate">
                          {actorId.substring(0, 12)}
                        </div>
                      )}
                    </td>
                    <td className="py-2 px-3 text-slate-400 font-sans truncate max-w-sm">
                      {ev.Actor?.Attributes ? (
                        <div className="flex flex-wrap gap-1">
                          {Object.entries(ev.Actor.Attributes)
                            .filter(([k]) => k !== 'name')
                            .slice(0, 3)
                            .map(([k, v]) => (
                              <span
                                key={k}
                                className="inline-block text-[10px] px-1.5 py-0.2 rounded bg-slate-800/80 text-slate-300 border border-slate-700/60 truncate max-w-[150px]"
                                title={`${k}=${v}`}
                              >
                                {k}={v}
                              </span>
                            ))}
                        </div>
                      ) : (
                        '-'
                      )}
                    </td>
                    <td className="py-2 px-4 text-right font-sans text-slate-400 whitespace-nowrap">
                      <div className="inline-flex items-center gap-1 text-slate-400">
                        <Server className="w-3 h-3 text-slate-500" />
                        <span>{ev.hostName}</span>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
