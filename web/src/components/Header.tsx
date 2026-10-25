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
    <header className="border-b border-slate-800 bg-slate-950/80 backdrop-blur sticky top-0 z-30 px-3 sm:px-6 py-2.5 sm:py-3.5">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5 sm:gap-3">
            <div className="p-1.5 sm:p-2 bg-sky-500/10 border border-sky-500/20 rounded-lg text-sky-400">
              <Shield className="w-4 h-4 sm:w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-base sm:text-lg font-semibold tracking-tight text-white">SandForge</h1>
                <span className="px-1.5 py-0.5 text-[10px] sm:text-xs font-medium rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                  v1.1.0
                </span>
              </div>
              <p className="text-[11px] sm:text-xs text-slate-400 line-clamp-1 sm:line-clamp-none">
                Ephemeral Execution Sandbox, CoW Rollback &amp; Egress Guard
              </p>
            </div>
          </div>

          {/* Mobile Refresh Button */}
          <div className="flex md:hidden items-center gap-2">
            <button
              onClick={onRefresh}
              className="p-1.5 rounded-md bg-slate-900 border border-slate-800 text-slate-400 hover:text-white"
              title="Refresh State"
            >
              <RefreshCw className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2 sm:gap-3">
          {/* Isolation Mode Badge */}
          <div className="flex items-center gap-1.5 px-2 sm:px-3 py-1 sm:py-1.5 rounded-md bg-slate-900 border border-slate-800 text-[11px] sm:text-xs">
            <Zap className="w-3 h-3 sm:w-3.5 sm:h-3.5 text-amber-400" />
            <span className="text-slate-400">Engine:</span>
            <span className="font-mono font-medium text-slate-200 uppercase">
              {status?.mode || 'LOCAL'}
            </span>
          </div>

          {/* Honeytoken Tripwires */}
          <div className="flex items-center gap-1.5 px-2 sm:px-3 py-1 sm:py-1.5 rounded-md bg-slate-900 border border-slate-800 text-[11px] sm:text-xs">
            <Bug className="w-3 h-3 sm:w-3.5 sm:h-3.5 text-purple-400" />
            <span className="text-slate-400 hidden xs:inline">Decoy Traps:</span>
            <span className="font-mono font-medium text-purple-300">
              {status?.firewall.honeytokens_active ?? 3} Active
            </span>
          </div>

          {/* Blocked Exfiltrations Counter */}
          <div className="flex items-center gap-1.5 px-2 sm:px-3 py-1 sm:py-1.5 rounded-md bg-slate-900 border border-slate-800 text-[11px] sm:text-xs">
            <Terminal className="w-3 h-3 sm:w-3.5 sm:h-3.5 text-red-400" />
            <span className="text-slate-400 hidden xs:inline">Blocked Exfils:</span>
            <span className={`font-mono font-semibold ${(status?.blocked_attempts || 0) > 0 ? 'text-red-400' : 'text-slate-300'}`}>
              {status?.blocked_attempts || 0}
            </span>
          </div>

          {/* Connection Status */}
          <div className="flex items-center gap-1.5 px-2 sm:px-3 py-1 sm:py-1.5 rounded-md bg-slate-900 border border-slate-800 text-[11px] sm:text-xs">
            <Radio className={`w-3 h-3 sm:w-3.5 sm:h-3.5 ${connected ? 'text-emerald-400 animate-pulse' : 'text-slate-500'}`} />
            <span className={connected ? 'text-emerald-400 font-medium' : 'text-slate-500 font-medium'}>
              {connected ? 'Live' : 'Offline'}
            </span>
          </div>

          {/* Audit Report Export */}
          <button
            onClick={onDownloadReport}
            className="flex items-center gap-1.5 px-2.5 sm:px-3 py-1 sm:py-1.5 rounded-md bg-sky-600/20 hover:bg-sky-600/30 text-sky-300 border border-sky-500/30 text-[11px] sm:text-xs font-medium transition"
            title="Download Incident Audit Report"
          >
            <Download className="w-3 h-3 sm:w-3.5 sm:h-3.5" />
            <span>Export Audit</span>
          </button>

          {/* Desktop Refresh Button */}
          <button
            onClick={onRefresh}
            className="hidden md:flex p-1.5 rounded-md bg-slate-900 border border-slate-800 text-slate-400 hover:text-white hover:bg-slate-800 transition"
            title="Refresh State"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
