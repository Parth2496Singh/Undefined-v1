import React, { useState, useEffect, useMemo } from 'react';
import { BrowserRouter, Routes, Route, Link, useLocation } from 'react-router-dom';
import * as api from './api';

function LogsModal({ isOpen, onClose, logs, title }) {
  if (!isOpen) return null;
  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
      <div className="bg-zinc-900 border border-zinc-700 rounded-lg shadow-2xl w-full max-w-4xl max-h-[80vh] flex flex-col">
        <div className="p-4 border-b border-zinc-800 flex justify-between items-center bg-zinc-950 rounded-t-lg">
          <h3 className="text-lg font-bold text-white">{title || 'Error Logs'}</h3>
          <button onClick={onClose} className="text-zinc-400 hover:text-white font-bold">✕</button>
        </div>
        <div className="p-4 overflow-auto flex-1 bg-black text-green-400 font-mono text-sm whitespace-pre-wrap">
          {logs || 'No logs available.'}
        </div>
      </div>
    </div>
  );
}

function Sidebar({ isAdmin }) {
  const location = useLocation();
  const navItems = [
    { path: '/settings', label: 'Identity & Trust', icon: '🔑' },
    { path: '/velzard', label: 'Velzard (Production)', icon: '🚀' },
    { path: '/zegion', label: 'Zegion (Ephemeral)', icon: '⚡' },
    { path: '/health', label: 'Platform Health', icon: '📊' },
  ];
  if (isAdmin) {
    navItems.push({ path: '/admin', label: 'Admin Panel 🛡️', icon: '👑' });
  }

  return (
    <aside className="w-64 bg-zinc-950 border-r border-zinc-800 h-screen sticky top-0 flex flex-col">
      <div className="p-6 flex items-center gap-3 border-b border-zinc-800">
        <div className="w-8 h-8 bg-indigo-600 rounded flex items-center justify-center font-bold text-white">V</div>
        <h1 className="text-xl font-bold tracking-tight text-white">Velzion</h1>
      </div>
      <nav className="flex-1 p-4 flex flex-col gap-2">
        {navItems.map(item => (
          <Link
            key={item.path}
            to={item.path}
            className={`flex items-center gap-3 px-4 py-3 rounded-md transition-colors font-medium text-sm ${
              location.pathname === item.path ? 'bg-indigo-600/10 text-indigo-400' : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200'
            }`}
          >
            <span>{item.icon}</span>
            {item.label}
          </Link>
        ))}
      </nav>
    </aside>
  );
}

function Settings() {
  const [iamRole, setIamRole] = useState('');

  const handleBindRole = async (e) => {
    e.preventDefault();
    await api.bindIAMRole(iamRole.trim()).catch(e => console.error(e));
    alert('IAM Role Vaulted Successfully');
  };

  return (
    <div className="max-w-2xl">
      <h2 className="text-2xl font-bold text-white mb-6">Identity & Trust</h2>
      <div className="bg-zinc-900 p-6 rounded-lg shadow-xl border border-zinc-800">
        <div className="mb-8 p-5 bg-zinc-950 rounded border border-zinc-800">
          <p className="text-sm text-zinc-400 mb-4 font-medium">1. Provision Trust Model in your AWS Account</p>
          <a 
            href="https://console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/create/review?templateURL=https://velzion-public-templates-12345.s3.amazonaws.com/velzion-trust.yaml&stackName=VelzionTrust" 
            target="_blank" rel="noreferrer"
            className="block text-center w-full bg-orange-600 text-white py-2.5 rounded hover:bg-orange-500 transition-colors text-sm font-bold shadow-lg">
            Launch CloudFormation Stack
          </a>
        </div>

        <form onSubmit={handleBindRole} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2 text-zinc-300">2. Vault Role ARN</label>
            <input 
              type="text" 
              value={iamRole} 
              onChange={e => setIamRole(e.target.value)} 
              className="w-full bg-zinc-950 border border-zinc-700 rounded p-3 text-sm text-white focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 outline-none" 
              placeholder="arn:aws:iam::123456789012:role/VelzionExecutionRole" 
            />
          </div>
          <button type="submit" className="w-full bg-indigo-600 text-white py-3 rounded hover:bg-indigo-500 transition-colors font-bold shadow-lg">Save & Vault</button>
        </form>
      </div>
    </div>
  );
}

function Velzard({ repos }) {
  const [deployments, setDeployments] = useState([]);
  const [repoUrl, setRepoUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [instanceType, setInstanceType] = useState('t3.small');
  const [volumeSize, setVolumeSize] = useState(30);
  
  const [modalLogs, setModalLogs] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);

  const fetchDeployments = () => {
    api.getDeployments().then(res => setDeployments(res.data || [])).catch(console.error);
  };

  useEffect(() => {
    fetchDeployments();
    const interval = setInterval(fetchDeployments, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleDeploy = async (e) => {
    e.preventDefault();
    await api.triggerDeploy({ repo_url: repoUrl, branch, instance_type: instanceType, volume_size: Number(volumeSize) });
    fetchDeployments();
  };

  const handleDestroy = async (id) => {
    await api.destroyDeployment(id);
    fetchDeployments();
  };

  const openLogs = (logs) => {
    setModalLogs(logs);
    setIsModalOpen(true);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <h2 className="text-2xl font-bold text-white">Velzard Production Hub</h2>
        <span className="bg-zinc-800 text-xs px-2 py-1 rounded text-zinc-400 font-medium">EC2 Orchestrator</span>
      </div>

      <form onSubmit={handleDeploy} className="grid grid-cols-1 md:grid-cols-12 gap-5 mb-8 bg-zinc-900 p-6 rounded-lg border border-zinc-800 shadow-xl">
        <div className="col-span-12 md:col-span-4">
          <label className="block text-xs text-zinc-400 mb-1.5 uppercase tracking-wider font-bold">Repository</label>
          <select 
            value={repoUrl} 
            onChange={e => setRepoUrl(e.target.value)} 
            className="w-full bg-zinc-950 border border-zinc-700 rounded p-2.5 text-sm text-white focus:border-indigo-500 outline-none" 
            required>
            <option value="">Select Repository...</option>
            {(repos || []).map(r => (
              <option key={r} value={`https://github.com/${r}`}>{r}</option>
            ))}
          </select>
        </div>
        <div className="col-span-6 md:col-span-2">
          <label className="block text-xs text-zinc-400 mb-1.5 uppercase tracking-wider font-bold">Branch</label>
          <input type="text" value={branch} onChange={e => setBranch(e.target.value)} placeholder="main" className="w-full bg-zinc-950 border border-zinc-700 rounded p-2.5 text-sm text-white focus:border-indigo-500 outline-none" />
        </div>
        <div className="col-span-6 md:col-span-2">
          <label className="block text-xs text-zinc-400 mb-1.5 uppercase tracking-wider font-bold">Instance</label>
          <select value={instanceType} onChange={e => setInstanceType(e.target.value)} className="w-full bg-zinc-950 border border-zinc-700 rounded p-2.5 text-sm text-white focus:border-indigo-500 outline-none">
            <option value="t3.micro">t3.micro</option>
            <option value="t3.small">t3.small</option>
            <option value="t3.medium">t3.medium</option>
            <option value="c5.large">c5.large</option>
          </select>
        </div>
        <div className="col-span-12 md:col-span-2">
          <label className="block text-xs text-zinc-400 mb-1.5 uppercase tracking-wider font-bold flex justify-between">
            <span>EBS</span>
            <span className="text-indigo-400">{volumeSize}GB</span>
          </label>
          <input type="range" min="10" max="100" step="10" value={volumeSize} onChange={e => setVolumeSize(e.target.value)} className="w-full mt-2.5 accent-indigo-600" />
        </div>
        <div className="col-span-12 md:col-span-2 flex items-end">
          <button type="submit" className="w-full bg-emerald-600 text-white py-2.5 rounded hover:bg-emerald-500 transition-colors font-bold text-sm shadow-lg">Deploy</button>
        </div>
      </form>

      <div className="bg-zinc-900 rounded-lg shadow-xl border border-zinc-800 overflow-hidden">
        <table className="w-full text-left text-sm border-collapse">
          <thead className="bg-zinc-950">
            <tr className="border-b border-zinc-800 text-zinc-400 uppercase tracking-wider text-xs">
              <th className="p-4 font-medium">Repository</th>
              <th className="p-4 font-medium">Hardware</th>
              <th className="p-4 font-medium">Status</th>
              <th className="p-4 font-medium text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-800">
            {(deployments || []).map(d => (
              <tr key={d.id} className="hover:bg-zinc-800/50 transition-colors">
                <td className="p-4 font-mono text-xs">{d.github_repo_url.replace('https://github.com/','')} <span className="text-zinc-500">({d.branch})</span></td>
                <td className="p-4 text-zinc-400">{d.instance_type}</td>
                <td className="p-4">
                  <span className={`px-2.5 py-1 rounded-full text-[10px] font-bold tracking-wider ${
                    d.status === 'RUNNING' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 
                    d.status === 'PROVISIONING' ? 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20 animate-pulse' :
                    d.status === 'DESTROYING' ? 'bg-orange-500/10 text-orange-400 border border-orange-500/20 animate-pulse' :
                    d.status === 'FAILED' || d.status === 'DESTROY_FAILED' ? 'bg-red-500/10 text-red-400 border border-red-500/20' :
                    'bg-zinc-800 text-zinc-400'
                  }`}>
                    {d.status}
                  </span>
                </td>
                <td className="p-4 text-right space-x-3">
                  {(d.status === 'FAILED' || d.status === 'DESTROY_FAILED') && (
                    <button onClick={() => openLogs(d.error_logs)} className="text-zinc-400 hover:text-white text-xs font-bold transition-colors underline decoration-zinc-600 underline-offset-2">Logs</button>
                  )}
                  <button onClick={() => handleDestroy(d.id)} className="text-red-400 hover:text-red-300 text-xs font-bold transition-colors">DESTROY</button>
                </td>
              </tr>
            ))}
            {!deployments?.length && <tr><td colSpan="4" className="p-12 text-center text-zinc-500 italic">No active production deployments</td></tr>}
          </tbody>
        </table>
      </div>
      <LogsModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} logs={modalLogs} title="Terraform Error Logs" />
    </div>
  );
}

function Zegion() {
  const [environments, setEnvironments] = useState([]);
  const [modalLogs, setModalLogs] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);

  const fetchEnvironments = () => {
    api.getEnvironments().then(res => setEnvironments(res.data || [])).catch(console.error);
  };

  useEffect(() => {
    fetchEnvironments();
    const interval = setInterval(fetchEnvironments, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleTerminate = async (id) => {
    await api.terminateEnvironment(id);
    fetchEnvironments();
  };

  const openLogs = (logs) => {
    setModalLogs(logs);
    setIsModalOpen(true);
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <h2 className="text-2xl font-bold text-white">Zegion Ephemeral Previews</h2>
        <span className="bg-zinc-800 text-xs px-2 py-1 rounded text-zinc-400 font-medium">Webhook Listener Active</span>
      </div>
      
      <div className="bg-zinc-900 rounded-lg shadow-xl border border-zinc-800 overflow-hidden">
        <table className="w-full text-left text-sm border-collapse">
          <thead className="bg-zinc-950">
            <tr className="border-b border-zinc-800 text-zinc-400 uppercase tracking-wider text-xs">
              <th className="p-4 font-medium">PR #</th>
              <th className="p-4 font-medium">Repository</th>
              <th className="p-4 font-medium">Status</th>
              <th className="p-4 font-medium text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-800">
            {(environments || []).map(e => (
              <tr key={e.id} className="hover:bg-zinc-800/50 transition-colors">
                <td className="p-4 font-mono text-indigo-400 font-bold">#{e.pr_number}</td>
                <td className="p-4">{e.github_repo_url.replace('https://github.com/','')}</td>
                <td className="p-4">
                  <span className={`px-2.5 py-1 rounded-full text-[10px] font-bold tracking-wider ${
                    e.status === 'BUILT' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 
                    e.status === 'PROVISIONING' ? 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20 animate-pulse' :
                    e.status === 'DESTROYING' ? 'bg-orange-500/10 text-orange-400 border border-orange-500/20 animate-pulse' :
                    e.status === 'FAILED' || e.status === 'DESTROY_FAILED' ? 'bg-red-500/10 text-red-400 border border-red-500/20' :
                    'bg-zinc-800 text-zinc-400'
                  }`}>
                    {e.status}
                  </span>
                </td>
                <td className="p-4 text-right space-x-4">
                  {(e.status === 'FAILED' || e.status === 'DESTROY_FAILED') && (
                    <button onClick={() => openLogs(e.error_logs)} className="text-zinc-400 hover:text-white text-xs font-bold transition-colors underline decoration-zinc-600 underline-offset-2">Logs</button>
                  )}
                  <button onClick={() => handleTerminate(e.id)} className="bg-red-500/10 text-red-500 border border-red-500/20 text-[10px] font-bold px-3 py-1.5 rounded hover:bg-red-500 hover:text-white transition-colors">
                    KILL SWITCH
                  </button>
                </td>
              </tr>
            ))}
            {!environments?.length && <tr><td colSpan="4" className="p-12 text-center text-zinc-500 italic">No ephemeral PR environments active</td></tr>}
          </tbody>
        </table>
      </div>
      <LogsModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} logs={modalLogs} title="Zegion Error Logs" />
    </div>
  );
}

function Health() {
  const [summary, setSummary] = useState(null);

  useEffect(() => {
    const fetchHealth = () => api.getTelemetrySummary().then(res => setSummary(res.data)).catch(console.error);
    fetchHealth();
    const int = setInterval(fetchHealth, 5000);
    return () => clearInterval(int);
  }, []);

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Platform Health & Telemetry</h2>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-zinc-900 p-6 rounded-lg shadow-xl border border-zinc-800 flex flex-col items-center justify-center text-center">
          <span className="text-zinc-400 text-sm font-bold uppercase tracking-wider mb-2">Active Clusters</span>
          <span className="text-5xl font-bold text-emerald-400">{summary?.active_nodes ?? '-'}</span>
        </div>
        <div className="bg-zinc-900 p-6 rounded-lg shadow-xl border border-zinc-800 flex flex-col items-center justify-center text-center">
          <span className="text-zinc-400 text-sm font-bold uppercase tracking-wider mb-2">Total Deployments</span>
          <span className="text-5xl font-bold text-indigo-400">{summary?.total_deployments ?? '-'}</span>
        </div>
        <div className="bg-zinc-900 p-6 rounded-lg shadow-xl border border-zinc-800 flex flex-col items-center justify-center text-center">
          <span className="text-zinc-400 text-sm font-bold uppercase tracking-wider mb-2">Engine Uptime</span>
          <span className="text-5xl font-bold text-orange-400">{summary?.uptime_seconds ? `${Math.floor(summary.uptime_seconds / 60)}m ${summary.uptime_seconds % 60}s` : '-'}</span>
        </div>
      </div>
    </div>
  );
}

function AdminDashboard() {
  const [tab, setTab] = useState('fleet');
  const [deployments, setDeployments] = useState([]);
  const [users, setUsers] = useState([]);

  const [systemSummary, setSystemSummary] = useState(null);

  useEffect(() => {
    if (tab === 'fleet') {
      api.getAdminDeployments().then(res => {
        setDeployments(Array.isArray(res.data) ? res.data : []);
      }).catch(console.error);
      api.getAdminSystemSummary().then(res => setSystemSummary(res.data)).catch(console.error);
    } else if (tab === 'users') {
      api.getAdminUsers().then(res => {
        setUsers(Array.isArray(res.data) ? res.data : []);
      }).catch(console.error);
    }
  }, [tab]);

  const handleTerminate = async (id, engine) => {
    if (engine === 'velzard') {
      await api.adminTerminateVelzard(id).catch(console.error);
    } else {
      await api.adminTerminateZegion(id).catch(console.error);
    }
    api.getAdminDeployments().then(res => setDeployments(res.data || []));
  };

  const handleReconcile = async () => {
    const res = await api.adminReconcile().catch(console.error);
    if (res) alert(`Reconciled ${res.data.flushed_count} orphaned deployments.`);
  };

  return (
    <div>
      <h2 className="text-2xl font-bold text-white mb-6">Master Control Plane</h2>
      <div className="flex gap-4 mb-6 border-b border-zinc-800 pb-2">
        <button onClick={() => setTab('fleet')} className={`font-bold pb-2 ${tab === 'fleet' ? 'text-indigo-400 border-b-2 border-indigo-400' : 'text-zinc-500'}`}>Fleet Control</button>
        <button onClick={() => setTab('users')} className={`font-bold pb-2 ${tab === 'users' ? 'text-indigo-400 border-b-2 border-indigo-400' : 'text-zinc-500'}`}>Users</button>
        <button onClick={() => setTab('db')} className={`font-bold pb-2 ${tab === 'db' ? 'text-indigo-400 border-b-2 border-indigo-400' : 'text-zinc-500'}`}>Database Ops</button>
        <button onClick={() => setTab('telemetry')} className={`font-bold pb-2 ${tab === 'telemetry' ? 'text-indigo-400 border-b-2 border-indigo-400' : 'text-zinc-500'}`}>Live Telemetry</button>
      </div>
      
      {tab === 'fleet' && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <div className="bg-zinc-900 p-4 rounded-lg border border-zinc-800 flex flex-col justify-center">
              <span className="text-zinc-500 text-xs font-bold uppercase tracking-wider mb-1">Total Users</span>
              <span className="text-3xl font-bold text-white">{systemSummary?.total_users ?? '-'}</span>
            </div>
            <div className="bg-zinc-900 p-4 rounded-lg border border-zinc-800 flex flex-col justify-center">
              <span className="text-zinc-500 text-xs font-bold uppercase tracking-wider mb-1">Successful Deployments</span>
              <span className="text-3xl font-bold text-emerald-400">{systemSummary?.total_successful_deployments ?? '-'}</span>
            </div>
            <div className="bg-zinc-900 p-4 rounded-lg border border-zinc-800 flex flex-col justify-center">
              <span className="text-zinc-500 text-xs font-bold uppercase tracking-wider mb-1">Failed Deployments</span>
              <span className="text-3xl font-bold text-red-400">{systemSummary?.total_failed_deployments ?? '-'}</span>
            </div>
            <div className="bg-zinc-900 p-4 rounded-lg border border-zinc-800 flex flex-col justify-center">
              <span className="text-zinc-500 text-xs font-bold uppercase tracking-wider mb-1">Terminations</span>
              <span className="text-3xl font-bold text-indigo-400">{systemSummary?.total_successful_terminations ?? '-'}</span>
            </div>
          </div>

          <div className="bg-zinc-900 rounded-lg shadow-xl border border-zinc-800 overflow-hidden">
            <table className="w-full text-left text-zinc-300">
              <thead className="bg-zinc-950 text-zinc-400 uppercase text-xs">
                <tr>
                  <th className="p-4">Engine</th>
                  <th className="p-4">Repo</th>
                  <th className="p-4">Context</th>
                  <th className="p-4">Audit Trail</th>
                  <th className="p-4">Status</th>
                  <th className="p-4 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800">
                {deployments.map(d => (
                  <tr key={d.id} className="hover:bg-zinc-800/50">
                    <td className="p-4 font-bold">
                      <span className={`px-2 py-1 rounded text-xs ${d.engine === 'velzard' ? 'bg-purple-500/20 text-purple-400 border-purple-500/30 border' : 'bg-orange-500/20 text-orange-400 border-orange-500/30 border'}`}>
                        {d.engine ? d.engine.toUpperCase() : 'UNKNOWN'}
                      </span>
                    </td>
                    <td className="p-4 font-mono text-xs text-zinc-300">{d.repo}</td>
                    <td className="p-4 font-mono text-xs text-zinc-400">{d.context}</td>
                    <td className="p-4 text-xs font-mono">
                      <div className="text-zinc-300 whitespace-nowrap">Deployed: {formatDeployTime(d.started_at)}</div>
                      <div className="text-zinc-500 whitespace-nowrap">Uptime: {formatUptime(d.started_at, d.destroyed_at)}</div>
                    </td>
                    <td className="p-4 text-xs font-bold">{d.status}</td>
                    <td className="p-4 text-right">
                      {(d.status === 'RUNNING' || d.status === 'PROVISIONING' || d.status === 'BUILT') && (
                        <button onClick={() => {
                          if (window.confirm('Force kill this deployment?')) {
                            const fn = d.engine === 'velzard' ? api.adminTerminateVelzard : api.adminTerminateZegion;
                            fn(d.id).then(() => alert('Kill signal sent')).catch(console.error);
                          }
                        }} className="text-red-500 hover:text-red-400 text-xs font-bold">FORCE KILL</button>
                      )}
                    </td>
                  </tr>
                ))}
                {!deployments.length && <tr><td colSpan="6" className="p-12 text-center text-zinc-500 italic">No deployments running</td></tr>}
              </tbody>
            </table>
          </div>
        </>
      )}
      
      {tab === 'users' && (
        <div className="bg-zinc-900 rounded-lg shadow-xl border border-zinc-800 overflow-hidden p-4 grid gap-4">
          {users.map(u => (
            <div key={u.ID} className="flex justify-between items-center bg-zinc-950 p-4 border border-zinc-800 rounded">
              <div>
                <span className="block font-bold text-white">{u.Username}</span>
                <span className="block text-xs font-mono text-zinc-500">{u.Email}</span>
              </div>
              <div className="text-right flex flex-col items-end gap-2">
                <span className="block text-xs text-indigo-400 font-mono">{u.IAMRoleARN || 'NO ROLE BOUND'}</span>
                <div>
                  {u.IsAdmin && <span className="text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-2 py-1 rounded mr-2 inline-block">ADMIN</span>}
                  <button onClick={() => { 
                    if(window.confirm('Revoke access and destroy all fleets for this user?')) { 
                      api.adminDeleteUser(u.ID).then(() => api.getAdminUsers().then(res => setUsers(Array.isArray(res.data) ? res.data : []))).catch(console.error);
                    } 
                  }} className="bg-red-500/10 text-red-500 border border-red-500/20 text-[10px] font-bold px-2 py-1 rounded hover:bg-red-500 hover:text-white transition-colors cursor-pointer">
                    Revoke Access
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
      
      {tab === 'db' && (
        <div className="bg-zinc-900 p-8 rounded-lg shadow-xl border border-zinc-800 text-center">
          <h3 className="text-xl font-bold text-red-500 mb-2">Danger Zone</h3>
          <p className="text-sm text-zinc-400 mb-6">This will forcefully flush any stuck PROVISIONING or DESTROYING rows to FAILED across all engines.</p>
          <button onClick={handleReconcile} className="bg-red-600 text-white font-bold py-3 px-6 rounded hover:bg-red-500 transition-colors shadow-lg shadow-red-600/20 mr-4">Reconcile System State</button>
          
          <div className="mt-12 pt-8 border-t border-red-500/20">
            <h3 className="text-xl font-bold text-red-500 mb-2">Nuclear Option</h3>
            <p className="text-sm text-zinc-400 mb-6">Wipes all fleets and users (except you). There is no going back.</p>
            <button onClick={async () => {
              const res = window.prompt("Type 'DELETE ALL' to execute a destructive factory reset of the system database.");
              if (res === 'DELETE ALL') {
                 await api.adminFlushSystem().catch(e => { alert('Factory reset failed'); throw e; });
                 alert('System factory reset successful');
                 window.location.reload();
              }
            }} className="bg-red-900/50 text-red-500 border border-red-900 font-bold py-3 px-6 rounded hover:bg-red-600 hover:text-white transition-colors">Factory Reset Database</button>
          </div>
        </div>
      )}

      {tab === 'telemetry' && (
        <div className="h-[800px]">
          <iframe src="http://localhost:3000/d/velzion-overview/velzion-overview?orgId=1&refresh=5s&kiosk" width="100%" height="100%" frameBorder="0" className="rounded-lg border border-zinc-800 shadow-xl"></iframe>
        </div>
      )}
    </div>
  );
}

function formatUptime(start, destroy) {
  if (!start) return '-';
  const s = new Date(start).getTime();
  const e = destroy ? new Date(destroy).getTime() : Date.now();
  const diff = Math.max(0, Math.floor((e - s) / 1000));
  const h = Math.floor(diff / 3600);
  const m = Math.floor((diff % 3600) / 60);
  return `${h}h ${m}m`;
}

function formatDeployTime(start) {
  if (!start) return '-';
  return new Date(start).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

function parseJwt(token) {
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch (e) {
    return null;
  }
}

export default function App() {
  const [token, setToken] = useState(localStorage.getItem('token') || '');
  const [repos, setRepos] = useState([]);

  const isAdmin = useMemo(() => {
    if (!token) return false;
    const payload = parseJwt(token);
    return payload ? payload.is_admin === true : false;
  }, [token]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const urlToken = params.get('token');
    
    if (urlToken) {
      setToken(urlToken);
      localStorage.setItem('token', urlToken);
      window.history.replaceState({}, document.title, window.location.pathname);
    }

    if (token || urlToken) {
      api.getRepos().then(res => setRepos(res.data)).catch(console.error);
    }
  }, [token]);

  const handleLogin = () => {
    window.location.href = "http://localhost:8081/api/auth/github/login";
  };

  if (!token) {
    return (
      <div className="min-h-screen bg-zinc-950 text-slate-200 flex flex-col items-center justify-center font-sans p-4">
        <div className="w-16 h-16 bg-indigo-600 rounded-xl flex items-center justify-center font-bold text-4xl text-white mb-8 shadow-2xl shadow-indigo-600/20">V</div>
        <h1 className="text-3xl font-bold tracking-tight text-white mb-8">Velzion Control Plane</h1>
        <button onClick={handleLogin} className="bg-emerald-600 text-white px-8 py-3 rounded-md hover:bg-emerald-500 font-bold transition-colors shadow-lg">
          Login with GitHub
        </button>
      </div>
    );
  }

  return (
    <BrowserRouter>
      <div className="min-h-screen bg-zinc-950 text-slate-200 font-sans flex">
        <Sidebar isAdmin={isAdmin} />
        <main className="flex-1">
          <header className="h-20 border-b border-zinc-800 bg-zinc-950/50 backdrop-blur-md sticky top-0 flex items-center justify-end px-8 z-40">
            <button onClick={() => { setToken(''); localStorage.removeItem('token'); }} className="text-sm font-bold text-zinc-400 hover:text-white transition-colors">
              Sign Out
            </button>
          </header>
          <div className="p-8 max-w-6xl">
            <Routes>
              <Route path="/" element={<Velzard repos={repos} />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="/velzard" element={<Velzard repos={repos} />} />
              <Route path="/zegion" element={<Zegion />} />
              <Route path="/health" element={<Health />} />
              {isAdmin && <Route path="/admin" element={<AdminDashboard />} />}
            </Routes>
          </div>
        </main>
      </div>
    </BrowserRouter>
  );
}