import React, { useState } from 'react';
import { History, RotateCcw, Eye, Clock, CheckCircle2 } from 'lucide-react';
import type { Snapshot } from '../types';

interface SnapshotTimelineProps {
  snapshots: Snapshot[];
  onRollback: (snapshotId: string) => Promise<void>;
  onInspectDiff: (snapshotId: string) => void;
  onRefresh: () => void;
}

export const SnapshotTimeline: React.FC<SnapshotTimelineProps> = ({
  snapshots,
  onRollback,
  onInspectDiff,
}) => {
  const [revertingId, setRevertingId] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const handleRollback = async (id: string) => {
    if (!window.confirm(`Are you sure you want to rollback to checkpoint ${id}? Any uncheckpointed agent work will be reverted.`)) {
      return;
    }

    try {
      setRevertingId(id);
      await onRollback(id);
      setMessage(`Successfully reverted workspace to ${id}`);
      setTimeout(() => setMessage(null), 3000);
    } catch (e: any) {
      alert(`Rollback failed: ${e.message || e}`);
    } finally {
      setRevertingId(null);
    }
  };

  const reversed = [...snapshots].reverse();

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 flex flex-col h-full">
      <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <History className="w-4 h-4 text-sky-400" />
          <h2 className="text-sm font-semibold text-white tracking-wide">
            Snapshot History & CoW Checkpoints
          </h2>
        </div>
        <span className="text-xs px-2.5 py-1 rounded-full bg-slate-800 text-slate-300 font-mono">
          {snapshots.length} Recorded
        </span>
      </div>

      {message && (
        <div className="mb-4 p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-lg text-emerald-400 text-xs flex items-center gap-2">
          <CheckCircle2 className="w-4 h-4" />
          {message}
        </div>
      )}

      {reversed.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-slate-500 border border-dashed border-slate-800 rounded-lg">
          <Clock className="w-8 h-8 mb-2 opacity-40 text-slate-400" />
          <p className="text-xs font-medium text-slate-400">No checkpoints recorded yet</p>
          <p className="text-[11px] text-slate-500 mt-1 max-w-xs">
            Commands executed via SandForge automatically capture a pre-execution checkpoint for instantaneous rollback.
          </p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto space-y-3 pr-1">
          {reversed.map((snap, index) => {
            const isReverting = revertingId === snap.id;
            const isLatest = index === 0;

            return (
              <div
                key={snap.id}
                className={`p-3.5 rounded-lg border transition ${
                  isLatest
                    ? 'bg-slate-800/40 border-sky-500/30'
                    : 'bg-slate-950/40 border-slate-800 hover:border-slate-700'
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs font-semibold text-sky-400">
                        {snap.id}
                      </span>
                      {isLatest && (
                        <span className="px-1.5 py-0.5 text-[10px] font-medium bg-sky-500/20 text-sky-300 rounded border border-sky-500/30">
                          Current
                        </span>
                      )}
                      <span className="text-[11px] text-slate-500 flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {new Date(snap.timestamp).toLocaleTimeString()}
                      </span>
                    </div>

                    <div className="font-mono text-xs text-slate-200 mt-1.5 truncate bg-slate-900/80 px-2 py-1 rounded border border-slate-800">
                      $ {snap.trigger_command}
                    </div>

                    {snap.description && (
                      <div className="text-[11px] text-slate-400 mt-1 italic">
                        {snap.description}
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-1.5 shrink-0">
                    <button
                      onClick={() => onInspectDiff(snap.id)}
                      className="flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition"
                      title="Inspect Changes"
                    >
                      <Eye className="w-3.5 h-3.5" />
                      Diff
                    </button>

                    <button
                      onClick={() => handleRollback(snap.id)}
                      disabled={isReverting}
                      className="flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-md bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border border-amber-500/30 transition disabled:opacity-50"
                      title="Rollback workspace to this snapshot"
                    >
                      <RotateCcw className={`w-3.5 h-3.5 ${isReverting ? 'animate-spin' : ''}`} />
                      {isReverting ? 'Rolling back...' : 'Revert'}
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
