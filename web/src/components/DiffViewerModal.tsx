import React from 'react';
import { X, FileCode, Plus, Minus, Copy, Check } from 'lucide-react';
import type { SnapshotDiff } from '../types';

interface DiffViewerModalProps {
  diff: SnapshotDiff | null;
  onClose: () => void;
}

export const DiffViewerModal: React.FC<DiffViewerModalProps> = ({ diff, onClose }) => {
  const [copied, setCopied] = React.useState(false);
  const [selectedFile, setSelectedFile] = React.useState<string | null>(null);

  if (!diff) return null;

  const activeEntry = diff.entries.find(e => e.path === selectedFile) || diff.entries[0];

  const handleCopy = () => {
    if (diff.raw) {
      navigator.clipboard.writeText(diff.raw);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="bg-slate-900 border border-slate-800 rounded-xl max-w-4xl w-full h-[80vh] flex flex-col shadow-2xl overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/50">
          <div>
            <h2 className="text-base font-semibold text-white flex items-center gap-2">
              <FileCode className="w-5 h-5 text-sky-400" />
              Filesystem Diff Inspection
            </h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Comparing checkpoint <span className="font-mono text-sky-400">{diff.from_id}</span> with <span className="font-mono text-slate-300">{diff.to_id}</span>
            </p>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={handleCopy}
              className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-md border border-slate-700 transition"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              {copied ? 'Copied' : 'Copy Raw Diff'}
            </button>
            <button
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-white rounded-md hover:bg-slate-800 transition"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Content area */}
        <div className="flex-1 flex overflow-hidden">
          {/* File sidebar */}
          <div className="w-72 border-r border-slate-800 overflow-y-auto bg-slate-950/30 p-2">
            <div className="text-xs font-semibold text-slate-400 uppercase tracking-wider px-3 py-2">
              Modified Files ({diff.entries.length})
            </div>
            {diff.entries.length === 0 ? (
              <div className="text-xs text-slate-500 px-3 py-4">No file modifications recorded.</div>
            ) : (
              <div className="space-y-1">
                {diff.entries.map((entry) => {
                  const isSelected = activeEntry?.path === entry.path;
                  return (
                    <button
                      key={entry.path}
                      onClick={() => setSelectedFile(entry.path)}
                      className={`w-full text-left px-3 py-2 rounded-lg text-xs font-mono transition flex items-center justify-between ${
                        isSelected
                          ? 'bg-sky-500/10 text-sky-400 border border-sky-500/30'
                          : 'text-slate-300 hover:bg-slate-800/60'
                      }`}
                    >
                      <span className="truncate" title={entry.path}>{entry.path}</span>
                      <div className="flex items-center gap-1.5 text-[10px] ml-2 shrink-0">
                        {entry.additions > 0 && (
                          <span className="text-emerald-400">+{entry.additions}</span>
                        )}
                        {entry.deletions > 0 && (
                          <span className="text-red-400">-{entry.deletions}</span>
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* Diff view */}
          <div className="flex-1 overflow-auto bg-slate-950 p-4 font-mono text-xs">
            {activeEntry?.patch ? (
              <pre className="space-y-0.5">
                {activeEntry.patch.split('\n').map((line, idx) => {
                  let lineClass = 'text-slate-300';
                  let bgClass = '';
                  let icon = null;

                  if (line.startsWith('+') && !line.startsWith('+++')) {
                    lineClass = 'text-emerald-400 font-medium';
                    bgClass = 'bg-emerald-950/30 border-l-2 border-emerald-500';
                    icon = <Plus className="w-3 h-3 text-emerald-500 inline mr-1" />;
                  } else if (line.startsWith('-') && !line.startsWith('---')) {
                    lineClass = 'text-red-400 font-medium';
                    bgClass = 'bg-red-950/30 border-l-2 border-red-500';
                    icon = <Minus className="w-3 h-3 text-red-500 inline mr-1" />;
                  } else if (line.startsWith('@@')) {
                    lineClass = 'text-sky-400 font-semibold bg-sky-950/20 py-0.5';
                  }

                  return (
                    <div key={idx} className={`px-2 py-0.5 rounded-sm flex items-center ${bgClass} ${lineClass}`}>
                      {icon}
                      <span className="whitespace-pre-wrap">{line}</span>
                    </div>
                  );
                })}
              </pre>
            ) : (
              <div className="h-full flex items-center justify-center text-slate-500">
                {diff.raw ? (
                  <pre className="text-slate-400 p-4">{diff.raw}</pre>
                ) : (
                  'No changes detected between these snapshots.'
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
