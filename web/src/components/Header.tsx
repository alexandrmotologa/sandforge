import React from 'react';
import { Shield, Zap, Terminal, RefreshCw, Radio, Download, Bug } from 'lucide-react';
import type { SandboxStatus } from '../types';

interface HeaderProps {
  status: SandboxStatus | null;
  connected: boolean;
  onRefresh: () => void;
  onDownloadReport: () => void;
}

export const Header: React.FC<HeaderProps> = ({ status, connected, onRefresh, onDownloadReport }) => {
  return (
    <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur sticky top-0 z-30 px-6 py-3.5">
      <div className="max-w-7xl mx-auto flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-sky-500/10 border border-sky-500/20 rounded-lg text-sky-400">
            <Shield className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-semibold tracking-tight text-white">SandForge</h1>
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                v1.1.0
              </span>
            </div>
            <p className="text-xs text-slate-400">
              Ephemeral Execution Sandbox, CoW Rollback & Egress Guard
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {/* Isolation Mode Badge */}
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-slate-900 border border-slate-800 text-xs">
            <Zap className="w-3.5 h-3.5 text-amber-400" />
            <span className="text-slate-400">Engine:</span>
            <span className="font-mono font-medium text-slate-200 uppercase">
              {status?.mode || 'LOCAL'}
            </span>
          </div>

          {/* Honeytoken Tripwires */}
          <div className="hidden sm:flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-slate-900 border border-slate-800 text-xs">
            <Bug className="w-3.5 h-3.5 text-purple-400" />
            <span className="text-slate-400">Decoy Traps:</span>
            <span className="font-mono font-medium text-purple-300">
              {status?.firewall.honeytokens_active ?? 3} Active
            </span>
          </div>

          {/* Blocked Exfiltrations Counter */}
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-slate-900 border border-slate-800 text-xs">
            <Terminal className="w-3.5 h-3.5 text-red-400" />
            <span className="text-slate-400">Blocked Exfils:</span>
            <span className={`font-mono font-semibold ${(status?.blocked_attempts || 0) > 0 ? 'text-red-400' : 'text-slate-300'}`}>
              {status?.blocked_attempts || 0}
            </span>
          </div>

          {/* Connection Status */}
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-slate-900 border border-slate-800 text-xs">
            <Radio className={`w-3.5 h-3.5 ${connected ? 'text-emerald-400 animate-pulse' : 'text-slate-500'}`} />
            <span className={connected ? 'text-emerald-400 font-medium' : 'text-slate-500 font-medium'}>
              {connected ? 'Live' : 'Disconnected'}
            </span>
          </div>

          {/* Download Incident Report */}
          <button
            onClick={onDownloadReport}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-sky-600/20 hover:bg-sky-600/30 text-sky-300 border border-sky-500/30 text-xs font-medium transition"
            title="Download Security Incident & Execution Audit Report"
          >
            <Download className="w-3.5 h-3.5" />
            Export Audit
          </button>

          <button
            onClick={onRefresh}
            className="p-1.5 text-slate-400 hover:text-white rounded-md hover:bg-slate-800 transition"
            title="Refresh status"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
