import React, { useState, useEffect, useRef } from 'react';
import { 
  Shield, 
  Users, 
  Activity, 
  AlertTriangle, 
  UserPlus, 
  KeyRound, 
  Ban, 
  CheckCircle2, 
  Trash2, 
  RefreshCw, 
  X, 
  Radio,
  Lock,
  User,
  Clock
} from 'lucide-react';

export default function AdminView({ currentUser }) {
  const [activeSubtab, setActiveSubtab] = useState('users'); // 'users' | 'logs' | 'danger'
  const [users, setUsers] = useState([]);
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [logs, setLogs] = useState([]);
  const [isStreaming, setIsStreaming] = useState(false);

  // Modals state
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [createError, setCreateError] = useState('');

  const [resetModalUser, setResetModalUser] = useState(null);
  const [resetPasswordVal, setResetPasswordVal] = useState('');
  const [resetError, setResetError] = useState('');

  const [wipeModalOpen, setWipeModalOpen] = useState(false);
  const [wipeConfirmInput, setWipeConfirmInput] = useState('');
  const [isWiping, setIsWiping] = useState(false);
  const [wipeMessage, setWipeMessage] = useState('');

  const eventSourceRef = useRef(null);

  // Fetch users
  const fetchUsers = async () => {
    setLoadingUsers(true);
    try {
      const res = await fetch('/api/admin/users');
      if (res.ok) {
        const data = await res.json();
        setUsers(data || []);
      }
    } catch (err) {
      console.error('Failed to fetch admin users:', err);
    } finally {
      setLoadingUsers(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  // Setup SSE for Live Audit Logs
  useEffect(() => {
    if (activeSubtab === 'logs') {
      setIsStreaming(true);
      const es = new EventSource('/api/admin/logs/stream');
      eventSourceRef.current = es;

      es.onmessage = (event) => {
        try {
          const logEntry = JSON.parse(event.data);
          setLogs((prev) => [logEntry, ...prev.slice(0, 99)]); // Keep last 100
        } catch (e) {
          console.error('Failed to parse SSE event data', e);
        }
      };

      es.onerror = () => {
        setIsStreaming(false);
      };

      return () => {
        es.close();
      };
    } else {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    }
  }, [activeSubtab]);

  // Toggle user disabled status
  const handleToggleStatus = async (user) => {
    try {
      const res = await fetch(`/api/admin/users/${user.id}/disable`, {
        method: 'POST',
      });
      if (res.ok) {
        fetchUsers();
      } else {
        const text = await res.text();
        alert(text || 'Failed to update user status');
      }
    } catch (err) {
      alert(err.message);
    }
  };

  // Create User
  const handleCreateUser = async (e) => {
    e.preventDefault();
    setCreateError('');

    if (newUsername.length < 3 || newPassword.length < 6) {
      setCreateError('Username must be at least 3 chars, password at least 6 chars.');
      return;
    }

    try {
      const res = await fetch('/api/admin/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: newUsername, password: newPassword }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to create user');
      }

      setCreateModalOpen(false);
      setNewUsername('');
      setNewPassword('');
      fetchUsers();
    } catch (err) {
      setCreateError(err.message);
    }
  };

  // Reset Password
  const handleResetPassword = async (e) => {
    e.preventDefault();
    setResetError('');

    if (resetPasswordVal.length < 6) {
      setResetError('Password must be at least 6 characters.');
      return;
    }

    try {
      const res = await fetch(`/api/admin/users/${resetModalUser.id}/reset_password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: resetPasswordVal }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to reset password');
      }

      setResetModalUser(null);
      setResetPasswordVal('');
      alert(`Password for ${resetModalUser.username} has been reset successfully.`);
    } catch (err) {
      setResetError(err.message);
    }
  };

  // Wipe Database
  const handleWipeDatabase = async () => {
    if (wipeConfirmInput !== 'wipe my data') return;
    setIsWiping(true);
    setWipeMessage('');

    try {
      const res = await fetch('/api/admin/wipe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ confirmation: 'wipe my data' }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to wipe database');
      }

      const result = await res.json();
      setWipeMessage(result.message || 'Database and file storage successfully wiped.');
      setWipeConfirmInput('');
      setWipeModalOpen(false);
      alert('System successfully wiped: All uploaded files and jobs permanently cleared.');
      fetchUsers();
    } catch (err) {
      alert(err.message);
    } finally {
      setIsWiping(false);
    }
  };

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2.5 mb-1">
            <Shield className="w-6 h-6 text-purple-400" />
            <h1 className="text-2xl sm:text-3xl font-bold text-white tracking-tight">Admin Control Panel</h1>
          </div>
          <p className="text-slate-400 text-sm">
            Privileged administrative operations, real-time audit event stream, and disaster recovery.
          </p>
        </div>
      </div>

      {/* Subtabs Bar */}
      <div className="flex items-center gap-2 p-1 bg-slate-900/80 rounded-xl border border-slate-800 w-fit">
        <button
          onClick={() => setActiveSubtab('users')}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-semibold transition cursor-pointer ${
            activeSubtab === 'users'
              ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/25'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Users className="w-4 h-4" />
          <span>User Management</span>
        </button>
        <button
          onClick={() => setActiveSubtab('logs')}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-semibold transition cursor-pointer ${
            activeSubtab === 'logs'
              ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/25'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          <Activity className="w-4 h-4" />
          <span>Live Audit Logs</span>
          <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
        </button>
        <button
          onClick={() => setActiveSubtab('danger')}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-semibold transition cursor-pointer ${
            activeSubtab === 'danger'
              ? 'bg-red-600 text-white shadow-md shadow-red-600/25'
              : 'text-slate-400 hover:text-red-400'
          }`}
        >
          <AlertTriangle className="w-4 h-4" />
          <span>Danger Zone</span>
        </button>
      </div>

      {/* TAB 1: User Management */}
      {activeSubtab === 'users' && (
        <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl rounded-2xl overflow-hidden shadow-xl">
          <div className="p-6 border-b border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h2 className="text-base font-bold text-white">System Accounts</h2>
              <p className="text-xs text-slate-400 mt-0.5">Control permissions, status, and credentials</p>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={fetchUsers}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition cursor-pointer"
                title="Refresh"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${loadingUsers ? 'animate-spin' : ''}`} />
              </button>
              <button
                onClick={() => setCreateModalOpen(true)}
                className="px-4 py-2 bg-gradient-to-r from-indigo-600 to-indigo-700 hover:from-indigo-500 hover:to-indigo-600 text-white text-xs font-semibold rounded-xl shadow-lg shadow-indigo-600/25 transition flex items-center gap-2 cursor-pointer"
              >
                <UserPlus className="w-3.5 h-3.5" />
                <span>Create New User</span>
              </button>
            </div>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm text-slate-300">
              <thead className="bg-slate-900/60 text-xs uppercase tracking-wider text-slate-400 font-semibold border-b border-slate-800/80">
                <tr>
                  <th className="px-6 py-3.5">ID</th>
                  <th className="px-6 py-3.5">Username</th>
                  <th className="px-6 py-3.5">Role</th>
                  <th className="px-6 py-3.5">Status</th>
                  <th className="px-6 py-3.5 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {users.map((u) => (
                  <tr key={u.id} className="hover:bg-slate-800/40 transition">
                    <td className="px-6 py-4 font-mono text-xs text-slate-500">#{u.id}</td>
                    <td className="px-6 py-4 font-semibold text-white">{u.username}</td>
                    <td className="px-6 py-4">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                        u.role === 'admin'
                          ? 'bg-purple-500/10 text-purple-400 border border-purple-500/20'
                          : 'bg-blue-500/10 text-blue-400 border border-blue-500/20'
                      }`}>
                        {u.role}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      {u.is_disabled ? (
                        <span className="text-xs text-red-400 font-medium">Disabled</span>
                      ) : (
                        <span className="text-xs text-emerald-400 font-medium">Active</span>
                      )}
                    </td>
                    <td className="px-6 py-4 text-right">
                      {u.role !== 'admin' && (
                        <div className="flex items-center justify-end gap-2">
                          <button
                            onClick={() => handleToggleStatus(u)}
                            className={`px-3 py-1 rounded-lg text-xs font-medium border transition cursor-pointer ${
                              u.is_disabled
                                ? 'bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border-emerald-500/20'
                                : 'bg-slate-800 hover:bg-slate-700 text-slate-300 border-slate-700/60'
                            }`}
                          >
                            {u.is_disabled ? 'Enable' : 'Disable'}
                          </button>
                          <button
                            onClick={() => {
                              setResetModalUser(u);
                              setResetPasswordVal('');
                              setResetError('');
                            }}
                            className="px-3 py-1 rounded-lg text-xs font-medium bg-indigo-600/10 hover:bg-indigo-600/20 text-indigo-400 border border-indigo-500/20 transition cursor-pointer"
                          >
                            Reset Password
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* TAB 2: Live Audit Logs */}
      {activeSubtab === 'logs' && (
        <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl rounded-2xl overflow-hidden shadow-xl p-6">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-3">
              <span className="relative flex h-3 w-3">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-500"></span>
              </span>
              <div>
                <h2 className="text-base font-bold text-white">Live Server-Sent Audit Logs</h2>
                <p className="text-xs text-slate-400">Streamed in real-time from Go API endpoint /api/admin/logs/stream</p>
              </div>
            </div>
            <button
              onClick={() => setLogs([])}
              className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition cursor-pointer"
            >
              Clear Logs
            </button>
          </div>

          <div className="bg-slate-950/80 border border-slate-800 rounded-xl p-4 font-mono text-xs max-h-[500px] overflow-y-auto space-y-2">
            {logs.length === 0 ? (
              <div className="p-8 text-center text-slate-500">
                Connected to SSE stream. Waiting for user actions or upload events...
              </div>
            ) : (
              logs.map((log, idx) => (
                <div
                  key={idx}
                  className="p-2.5 rounded-lg bg-slate-900/60 border border-slate-800/60 flex items-start gap-3 hover:bg-slate-900 transition"
                >
                  <span className="text-slate-500 whitespace-nowrap text-[11px]">
                    {new Date(log.created_at || log.timestamp || Date.now()).toLocaleTimeString()}
                  </span>
                  <span className="px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 text-[10px] font-semibold whitespace-nowrap">
                    {log.action || 'system_event'}
                  </span>
                  <div className="flex-1 min-w-0">
                    <span className="text-slate-400">Actor #{log.actor_id ?? log.user_id ?? 'system'}: </span>
                    <span className="text-slate-200 break-all">{JSON.stringify(log.details || {})}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* TAB 3: Danger Zone */}
      {activeSubtab === 'danger' && (
        <div className="bg-red-950/20 border border-red-500/30 backdrop-blur-xl rounded-2xl p-6 sm:p-8 shadow-xl">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-2xl bg-red-500/10 text-red-400 border border-red-500/20 flex items-center justify-center flex-shrink-0">
              <AlertTriangle className="w-6 h-6" />
            </div>
            <div className="flex-1">
              <h2 className="text-lg font-bold text-white">Disaster Recovery & Database Wipe</h2>
              <p className="text-sm text-slate-400 mt-1 max-w-2xl">
                Permanently purge all MongoDB documents, uploaded batches, jobs catalog, and storage files in 
                <code className="bg-slate-900 px-1.5 py-0.5 rounded text-red-400 font-mono ml-1">./uploads</code>.
                This action is irreversible and leaves no backups.
              </p>

              <div className="mt-6 pt-6 border-t border-red-500/20 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <span className="text-xs text-red-400 font-mono font-semibold">
                  Requires explicit confirmation: "wipe my data"
                </span>
                <button
                  onClick={() => setWipeModalOpen(true)}
                  className="px-5 py-2.5 bg-red-600 hover:bg-red-700 text-white text-xs font-bold rounded-xl shadow-lg shadow-red-600/30 transition flex items-center gap-2 cursor-pointer"
                >
                  <Trash2 className="w-4 h-4" />
                  <span>Wipe All System Data</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Modal: Create User */}
      {createModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-[#111726] border border-slate-800 rounded-2xl p-6 max-w-md w-full shadow-2xl">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-base font-bold text-white">Create New User</h3>
              <button onClick={() => setCreateModalOpen(false)} className="text-slate-400 hover:text-slate-200">
                <X className="w-5 h-5" />
              </button>
            </div>

            {createError && (
              <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
                {createError}
              </div>
            )}

            <form onSubmit={handleCreateUser} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Username</label>
                <input
                  type="text"
                  required
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  placeholder="Min 3 characters"
                  className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-200 text-sm focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">Password</label>
                <input
                  type="password"
                  required
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Min 6 characters"
                  className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-200 text-sm focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setCreateModalOpen(false)}
                  className="px-4 py-2 border border-slate-800 rounded-xl text-slate-300 text-xs hover:bg-slate-800 cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl text-xs font-semibold cursor-pointer"
                >
                  Create Account
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Reset Password */}
      {resetModalUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-[#111726] border border-slate-800 rounded-2xl p-6 max-w-md w-full shadow-2xl">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-base font-bold text-white">Reset Password for {resetModalUser.username}</h3>
              <button onClick={() => setResetModalUser(null)} className="text-slate-400 hover:text-slate-200">
                <X className="w-5 h-5" />
              </button>
            </div>

            {resetError && (
              <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
                {resetError}
              </div>
            )}

            <form onSubmit={handleResetPassword} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 uppercase mb-1">New Password</label>
                <input
                  type="password"
                  required
                  value={resetPasswordVal}
                  onChange={(e) => setResetPasswordVal(e.target.value)}
                  placeholder="Min 6 characters"
                  className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-200 text-sm focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setResetModalUser(null)}
                  className="px-4 py-2 border border-slate-800 rounded-xl text-slate-300 text-xs hover:bg-slate-800 cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl text-xs font-semibold cursor-pointer"
                >
                  Update Password
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Wipe Confirmation */}
      {wipeModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-[#111726] border border-red-500/40 rounded-2xl p-6 max-w-md w-full shadow-2xl">
            <div className="flex items-center gap-3 mb-4 text-red-400">
              <AlertTriangle className="w-6 h-6" />
              <h3 className="text-base font-bold text-white">Permanent Data Wipe</h3>
            </div>
            <p className="text-xs text-slate-400 mb-4 leading-relaxed">
              This will permanently delete all uploaded files from disk, erase jobs in MongoDB, and clear audit logs.
              To confirm, type <strong className="text-red-400 font-mono">wipe my data</strong> below:
            </p>
            <input
              type="text"
              value={wipeConfirmInput}
              onChange={(e) => setWipeConfirmInput(e.target.value)}
              placeholder="wipe my data"
              className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-xl text-slate-200 text-sm font-mono focus:outline-none focus:border-red-500 mb-4"
            />
            <div className="flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={() => { setWipeModalOpen(false); setWipeConfirmInput(''); }}
                className="px-4 py-2 border border-slate-800 rounded-xl text-slate-300 text-xs hover:bg-slate-800 cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={wipeConfirmInput !== 'wipe my data' || isWiping}
                onClick={handleWipeDatabase}
                className="px-5 py-2 bg-red-600 hover:bg-red-700 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-xl text-xs font-bold cursor-pointer flex items-center gap-2"
              >
                {isWiping ? (
                  <>
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                    <span>Wiping...</span>
                  </>
                ) : (
                  <span>Confirm Wipe</span>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
