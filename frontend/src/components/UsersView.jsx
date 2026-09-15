import React, { useState, useEffect } from 'react';
import { Users, Shield, RefreshCw, UserCheck, UserX, Clock, ArrowRight } from 'lucide-react';

export default function UsersView({ user, onNavigate }) {
  const [usersList, setUsersList] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      // In Poneglyph, /api/admin/users lists users. If non-admin, show current user profile info.
      const res = await fetch('/api/admin/users');
      if (res.ok) {
        const data = await res.json();
        setUsersList(data || []);
      } else {
        // Non-admin fallback view
        setUsersList([{
          id: user.id || 1,
          username: user.username,
          role: user.role,
          is_disabled: false,
          created_at: new Date().toISOString()
        }]);
      }
    } catch (err) {
      console.error('Failed to load users:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold text-white tracking-tight">Users & Directory</h1>
          <p className="text-slate-400 text-sm mt-1">
            Registered accounts, permissions, and security roles.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={fetchUsers}
            disabled={loading}
            className="p-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition cursor-pointer"
            title="Refresh list"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          {user?.role === 'admin' && (
            <button
              onClick={() => onNavigate('admin')}
              className="px-4 py-2.5 bg-gradient-to-r from-indigo-600 to-indigo-700 hover:from-indigo-500 hover:to-indigo-600 text-white text-sm font-semibold rounded-xl shadow-lg shadow-indigo-600/25 transition flex items-center gap-2 cursor-pointer"
            >
              <Shield className="w-4 h-4" />
              <span>Admin Management</span>
            </button>
          )}
        </div>
      </div>

      {/* Users Table */}
      <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl rounded-2xl overflow-hidden shadow-xl">
        <div className="p-6 border-b border-slate-800/80 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <Users className="w-4 h-4 text-indigo-400" />
            <h2 className="text-base font-bold text-white">Registered Accounts</h2>
          </div>
          <span className="text-xs text-slate-500 font-mono">
            {usersList.length} User(s) Total
          </span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm text-slate-300">
            <thead className="bg-slate-900/60 text-xs uppercase tracking-wider text-slate-400 font-semibold border-b border-slate-800/80">
              <tr>
                <th className="px-6 py-3.5">User ID</th>
                <th className="px-6 py-3.5">Username</th>
                <th className="px-6 py-3.5">Role</th>
                <th className="px-6 py-3.5">Status</th>
                <th className="px-6 py-3.5">Registered</th>
                {user?.role === 'admin' && (
                  <th className="px-6 py-3.5 text-right">Actions</th>
                )}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {loading ? (
                <tr>
                  <td colSpan={user?.role === 'admin' ? 6 : 5} className="px-6 py-12 text-center text-slate-500">
                    Loading users directory...
                  </td>
                </tr>
              ) : (
                usersList.map((u) => {
                  const regDate = u.created_at ? new Date(u.created_at).toLocaleDateString() : 'N/A';
                  return (
                    <tr key={u.id} className="hover:bg-slate-800/40 transition">
                      <td className="px-6 py-4 font-mono text-xs text-slate-500">
                        #{u.id}
                      </td>
                      <td className="px-6 py-4 font-semibold text-white flex items-center gap-2">
                        <div className="w-8 h-8 rounded-full bg-slate-800 text-indigo-400 flex items-center justify-center font-bold text-xs uppercase border border-slate-700/60">
                          {u.username?.[0] || 'U'}
                        </div>
                        <span>{u.username}</span>
                        {u.username === user?.username && (
                          <span className="text-[10px] bg-indigo-500/10 text-indigo-400 px-2 py-0.5 rounded border border-indigo-500/20">
                            You
                          </span>
                        )}
                      </td>
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
                          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-500/10 text-red-400 border border-red-500/20">
                            <UserX className="w-3 h-3" />
                            <span>Disabled</span>
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            <UserCheck className="w-3 h-3" />
                            <span>Active</span>
                          </span>
                        )}
                      </td>
                      <td className="px-6 py-4 text-xs text-slate-500 font-mono">
                        {regDate}
                      </td>
                      {user?.role === 'admin' && (
                        <td className="px-6 py-4 text-right">
                          <button
                            onClick={() => onNavigate('admin')}
                            className="text-xs text-indigo-400 hover:text-indigo-300 font-semibold transition cursor-pointer"
                          >
                            Manage →
                          </button>
                        </td>
                      )}
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
