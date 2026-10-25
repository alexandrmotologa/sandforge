import { useState } from 'react';
import { Header } from './components/Header';
import { ProcessMonitor } from './components/ProcessMonitor';
import { SnapshotTimeline } from './components/SnapshotTimeline';
import { EgressLogTable } from './components/EgressLogTable';
import { TerminalConsole } from './components/TerminalConsole';
import { DiffViewerModal } from './components/DiffViewerModal';
import { useSandboxStream } from './hooks/useSandboxStream';
import type { SnapshotDiff } from './types';

export function App() {
  const {
    status,
    snapshots,
    alerts,
    firewall,
    executions,
    connected,
    rollback,
    runCommand,
    fetchDiff,
    addDomain,
    removeDomain,
    refresh,
  } = useSandboxStream();

  const [activeDiff, setActiveDiff] = useState<SnapshotDiff | null>(null);

  const handleInspectDiff = async (snapshotId: string) => {
    try {
      const diff = await fetchDiff(snapshotId, 'working');
      setActiveDiff(diff);
    } catch (e: any) {
      alert(`Failed to load diff: ${e.message || e}`);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-sky-500/30 selection:text-sky-200">
      <Header
        status={status}
        connected={connected}
        onRefresh={refresh}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto p-6 flex flex-col">
        <ProcessMonitor status={status} />

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6 min-h-[420px]">
          <SnapshotTimeline
            snapshots={snapshots}
            onRollback={rollback}
            onInspectDiff={handleInspectDiff}
            onRefresh={refresh}
          />

          <EgressLogTable
            alerts={alerts}
            firewall={firewall}
            onAddDomain={addDomain}
            onRemoveDomain={removeDomain}
          />
        </div>

        <div className="min-h-[340px] flex-1">
          <TerminalConsole
            executions={executions}
            onRunCommand={runCommand}
            onRollback={rollback}
          />
        </div>
      </main>

      <footer className="border-t border-slate-900 py-3 text-center text-xs text-slate-600 bg-slate-950">
        SandForge Local Engine &bull; Sub-150ms Container Sandbox &bull; Sub-15ms CoW Rollback &bull; Prompt Injection Egress Shield
      </footer>

      {activeDiff && (
        <DiffViewerModal
          diff={activeDiff}
          onClose={() => setActiveDiff(null)}
        />
      )}
    </div>
  );
}

export default App;
