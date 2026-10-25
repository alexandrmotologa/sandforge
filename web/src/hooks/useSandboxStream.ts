import { useState, useEffect, useCallback, useRef } from 'react';
import type { SandboxStatus, Snapshot, NetworkAlert, ExecutionResult, SnapshotDiff, FirewallConfig, PolicyConfig } from '../types';

export function useSandboxStream() {
  const [status, setStatus] = useState<SandboxStatus | null>(null);
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [alerts, setAlerts] = useState<NetworkAlert[]>([]);
  const [firewall, setFirewall] = useState<FirewallConfig | null>(null);
  const [policy, setPolicy] = useState<PolicyConfig | null>(null);
  const [executions, setExecutions] = useState<ExecutionResult[]>([]);
  const [connected, setConnected] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);

  const getApiUrl = (path: string) => {
    const isDev = window.location.port === '5173';
    const baseUrl = isDev ? 'http://localhost:8920' : '';
    return `${baseUrl}${path}`;
  };

  const getWsUrl = () => {
    const isDev = window.location.port === '5173';
    if (isDev) {
      return 'ws://localhost:8920/api/ws';
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/api/ws`;
  };

  const fetchInitialData = useCallback(async () => {
    try {
      const [statusRes, snapRes, alertsRes, rulesRes, policyRes] = await Promise.all([
        fetch(getApiUrl('/api/status')).then(r => r.ok ? r.json() : null),
        fetch(getApiUrl('/api/snapshots')).then(r => r.ok ? r.json() : []),
        fetch(getApiUrl('/api/firewall/alerts?limit=100')).then(r => r.ok ? r.json() : []),
        fetch(getApiUrl('/api/firewall/rules')).then(r => r.ok ? r.json() : null),
        fetch(getApiUrl('/api/policy')).then(r => r.ok ? r.json() : null),
      ]);

      if (statusRes) setStatus(statusRes);
      if (snapRes) setSnapshots(snapRes);
      if (alertsRes) setAlerts(alertsRes);
      if (rulesRes) setFirewall(rulesRes);
      if (policyRes) setPolicy(policyRes);
    } catch (e) {
      console.warn('Failed to fetch initial status, server may be starting...', e);
    }
  }, []);

  useEffect(() => {
    fetchInitialData();

    let timeoutId: any;
    const connectWS = () => {
      try {
        const ws = new WebSocket(getWsUrl());
        wsRef.current = ws;

        ws.onopen = () => {
          setConnected(true);
        };

        ws.onmessage = (event) => {
          try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'alert') {
              setAlerts(prev => [msg.data, ...prev].slice(0, 200));
              fetchInitialData();
            } else if (msg.type === 'snapshot') {
              setSnapshots(prev => [...prev, msg.data]);
              fetchInitialData();
            } else if (msg.type === 'execution') {
              setExecutions(prev => [msg.data, ...prev].slice(0, 50));
              fetchInitialData();
            } else if (msg.type === 'rollback') {
              fetchInitialData();
            }
          } catch (e) {
            console.error('Failed to parse WS payload', e);
          }
        };

        ws.onclose = () => {
          setConnected(false);
          timeoutId = setTimeout(connectWS, 2000);
        };

        ws.onerror = () => {
          ws.close();
        };
      } catch (e) {
        timeoutId = setTimeout(connectWS, 2000);
      }
    };

    connectWS();

    return () => {
      clearTimeout(timeoutId);
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [fetchInitialData]);

  const rollback = async (snapshotId: string) => {
    const res = await fetch(getApiUrl('/api/snapshots/rollback'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ snapshot_id: snapshotId }),
    });
    if (!res.ok) {
      const err = await res.text();
      throw new Error(err);
    }
    await fetchInitialData();
  };

  const runCommand = async (command: string): Promise<ExecutionResult> => {
    const res = await fetch(getApiUrl('/api/exec'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command }),
    });
    if (!res.ok) {
      const err = await res.text();
      throw new Error(err);
    }
    const result: ExecutionResult = await res.json();
    setExecutions(prev => [result, ...prev]);
    await fetchInitialData();
    return result;
  };

  const runWithVerify = async (command: string, verifyCommand: string): Promise<ExecutionResult> => {
    const res = await fetch(getApiUrl('/api/exec/verify'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command, verify_command: verifyCommand }),
    });
    if (!res.ok) {
      const err = await res.text();
      throw new Error(err);
    }
    const result: ExecutionResult = await res.json();
    setExecutions(prev => [result, ...prev]);
    await fetchInitialData();
    return result;
  };

  const updatePolicy = async (newPolicy: PolicyConfig) => {
    const res = await fetch(getApiUrl('/api/policy'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newPolicy),
    });
    if (res.ok) {
      const updated = await res.json();
      setPolicy(updated);
    }
  };

  const fetchDiff = async (fromId: string, toId: string = 'working'): Promise<SnapshotDiff> => {
    const res = await fetch(getApiUrl(`/api/snapshots/diff?from=${encodeURIComponent(fromId)}&to=${encodeURIComponent(toId)}`));
    if (!res.ok) {
      const err = await res.text();
      throw new Error(err);
    }
    return res.json();
  };

  const addDomain = async (domain: string) => {
    const res = await fetch(getApiUrl('/api/firewall/rules'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain }),
    });
    if (res.ok) {
      const updated = await res.json();
      setFirewall(updated);
    }
  };

  const removeDomain = async (domain: string) => {
    const res = await fetch(getApiUrl('/api/firewall/rules'), {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ domain }),
    });
    if (res.ok) {
      const updated = await res.json();
      setFirewall(updated);
    }
  };

  const downloadAuditReport = (format: string = 'html') => {
    window.open(getApiUrl(`/api/audit/report?format=${format}`), '_blank');
  };

  return {
    status,
    snapshots,
    alerts,
    firewall,
    policy,
    executions,
    connected,
    rollback,
    runCommand,
    runWithVerify,
    updatePolicy,
    fetchDiff,
    addDomain,
    removeDomain,
    downloadAuditReport,
    refresh: fetchInitialData,
  };
}
