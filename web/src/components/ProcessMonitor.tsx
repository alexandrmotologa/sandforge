import React from 'react';
import { Cpu, Server, ShieldCheck, Zap } from 'lucide-react';
import type { SandboxStatus } from '../types';

interface ProcessMonitorProps {
  status: SandboxStatus | null;
}

export const ProcessMonitor: React.FC<ProcessMonitorProps> = ({ status }) => {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      {/* Engine Mode */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-slate-400">Sandbox Isolation</p>
          <div className="flex items-center gap-2 mt-1">
            <span className="text-xl font-bold font-mono text-white uppercase">
              {status?.mode || 'LOCAL'}
            </span>
            <span className="px-1.5 py-0.5 text-[10px] font-semibold rounded bg-sky-500/20 text-sky-400 border border-sky-500/30">
              Active
            </span>
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Sub-150ms execution runtime</p>
        </div>
        <div className="p-3 bg-slate-800 rounded-lg text-sky-400">
          <Server className="w-5 h-5" />
        </div>
      </div>

      {/* Pre-Warmed Worker Pool */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-slate-400">Pre-Warmed Pool</p>
          <div className="flex items-center gap-2 mt-1">
            <span className="text-xl font-bold font-mono text-white">
              {status?.idle_containers ?? 2} Idle
            </span>
            <span className="text-xs text-slate-400 font-mono">
              / {status?.active_containers ?? 0} Running
            </span>
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Standby capacity ready</p>
        </div>
        <div className="p-3 bg-slate-800 rounded-lg text-amber-400">
          <Zap className="w-5 h-5" />
        </div>
      </div>

      {/* Rollback Speed */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-slate-400">Rollback Latency</p>
          <div className="flex items-center gap-2 mt-1">
            <span className="text-xl font-bold font-mono text-emerald-400">&lt; 15 ms</span>
            <span className="px-1.5 py-0.5 text-[10px] font-semibold rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
              Instant
            </span>
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Git index tree checkpoint</p>
        </div>
        <div className="p-3 bg-slate-800 rounded-lg text-emerald-400">
          <Cpu className="w-5 h-5" />
        </div>
      </div>

      {/* Egress Security Guard */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-slate-400">Exfiltration Defense</p>
          <div className="flex items-center gap-2 mt-1">
            <span className="text-xl font-bold font-mono text-white">
              {status?.blocked_attempts ?? 0}
            </span>
            <span className="text-xs text-slate-400">
              Blocked Threats
            </span>
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Private IPs and metadata blocked</p>
        </div>
        <div className="p-3 bg-slate-800 rounded-lg text-sky-400">
          <ShieldCheck className="w-5 h-5" />
        </div>
      </div>
    </div>
  );
};
