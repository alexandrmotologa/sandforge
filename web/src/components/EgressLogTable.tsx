import React, { useState } from 'react';
import { ShieldCheck, ShieldAlert, Plus, Trash2, Globe, AlertTriangle } from 'lucide-react';
import type { NetworkAlert, FirewallConfig } from '../types';

interface EgressLogTableProps {
  alerts: NetworkAlert[];
  firewall: FirewallConfig | null;
  onAddDomain: (domain: string) => Promise<void>;
  onRemoveDomain: (domain: string) => Promise<void>;
}

export const EgressLogTable: React.FC<EgressLogTableProps> = ({
  alerts,
  firewall,
  onAddDomain,
  onRemoveDomain,
}) => {
  const [newDomain, setNewDomain] = useState('');
  const [showAllowlist, setShowAllowlist] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDomain.trim()) return;

    try {
      setSubmitting(true);
      await onAddDomain(newDomain.trim());
      setNewDomain('');
    } catch (e: any) {
      alert(`Failed to add domain: ${e.message || e}`);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 flex flex-col h-full">
      <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <ShieldAlert className="w-4 h-4 text-amber-400" />
          <h2 className="text-sm font-semibold text-white tracking-wide">
            Egress Network & Secret Guard Log
          </h2>
        </div>

        <button
          onClick={() => setShowAllowlist(!showAllowlist)}
          className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md bg-slate-800 hover:bg-slate-700 text-sky-400 border border-slate-700 transition"
        >
          <Globe className="w-3.5 h-3.5" />
          {showAllowlist ? 'Hide Allowlist' : 'Manage Allowlist'}
        </button>
      </div>

      {showAllowlist && (
        <div className="mb-4 p-4 rounded-lg bg-slate-950/70 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-xs font-semibold text-slate-300">
              Trusted Domain Allowlist ({firewall?.allowed_domains.length || 0})
            </h3>
            <span className="text-[11px] text-slate-500">
              Traffic to unknown domains or cloud metadata is blocked by default.
            </span>
          </div>

          <form onSubmit={handleAdd} className="flex gap-2">
            <input
              type="text"
              value={newDomain}
              onChange={(e) => setNewDomain(e.target.value)}
              placeholder="e.g., api.myservice.com"
              className="flex-1 bg-slate-900 border border-slate-700 rounded-md px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-sky-500"
            />
            <button
              type="submit"
              disabled={submitting || !newDomain.trim()}
              className="flex items-center gap-1 px-3 py-1.5 bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white rounded-md text-xs font-medium transition"
            >
              <Plus className="w-3.5 h-3.5" />
              Add Domain
            </button>
          </form>

          <div className="max-h-40 overflow-y-auto grid grid-cols-2 sm:grid-cols-3 gap-1.5 pt-1">
            {firewall?.allowed_domains.map((domain) => (
              <div
                key={domain}
                className="flex items-center justify-between px-2.5 py-1 bg-slate-900 border border-slate-800 rounded text-xs font-mono text-slate-300"
              >
                <span className="truncate" title={domain}>{domain}</span>
                <button
                  onClick={() => onRemoveDomain(domain)}
                  className="text-slate-500 hover:text-red-400 ml-1.5 transition"
                  title="Remove from allowlist"
                >
                  <Trash2 className="w-3 h-3" />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {alerts.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-slate-500 border border-dashed border-slate-800 rounded-lg">
          <ShieldCheck className="w-8 h-8 mb-2 opacity-40 text-emerald-400" />
          <p className="text-xs font-medium text-slate-400">No outbound egress requests detected</p>
          <p className="text-[11px] text-slate-500 mt-1 max-w-xs">
            When coding agent processes run curl, pip, or npm, outbound network attempts are evaluated against your domain allowlist.
          </p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto">
          <table className="w-full text-left text-xs">
            <thead className="text-[11px] uppercase tracking-wider text-slate-400 border-b border-slate-800 bg-slate-950/40 sticky top-0">
              <tr>
                <th className="py-2.5 px-3">Decision</th>
                <th className="py-2.5 px-3">Target Host</th>
                <th className="py-2.5 px-3">Port</th>
                <th className="py-2.5 px-3">Process</th>
                <th className="py-2.5 px-3">Reason</th>
                <th className="py-2.5 px-3 text-right">Time</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono">
              {alerts.map((a) => {
                const isBlocked = a.action === 'BLOCK';
                return (
                  <tr
                    key={a.id}
                    className={`hover:bg-slate-800/40 transition ${
                      isBlocked ? 'bg-red-950/20' : ''
                    }`}
                  >
                    <td className="py-2 px-3 whitespace-nowrap">
                      {isBlocked ? (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-red-500/20 text-red-400 border border-red-500/40 animate-pulse">
                          <AlertTriangle className="w-3 h-3" />
                          BLOCKED
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                          <ShieldCheck className="w-3 h-3" />
                          ALLOWED
                        </span>
                      )}
                    </td>

                    <td className="py-2 px-3 font-semibold text-slate-200 truncate max-w-xs" title={a.target_host}>
                      {a.target_host}
                    </td>

                    <td className="py-2 px-3 text-slate-400">{a.target_port}</td>
                    <td className="py-2 px-3 text-slate-300">{a.process}</td>
                    <td className="py-2 px-3 text-slate-400 font-sans text-[11px] truncate max-w-sm" title={a.reason}>
                      {a.reason}
                    </td>
                    <td className="py-2 px-3 text-right text-slate-500 whitespace-nowrap">
                      {new Date(a.timestamp).toLocaleTimeString()}
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
