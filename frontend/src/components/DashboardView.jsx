import React, { useState, useEffect } from 'react';
import { 
  FileText, 
  Layers, 
  HardDrive, 
  Users, 
  UploadCloud, 
  ArrowUpRight, 
  Clock, 
  ShieldCheck, 
  CheckCircle2, 
  Activity,
  RefreshCw
} from 'lucide-react';
import logoImg from '../assets/logo.png';

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export default function DashboardView({ user, onNavigate }) {
  const [stats, setStats] = useState({
    total_jobs: 0,
    total_files: 0,
    total_bytes: 0,
    total_users: 0,
    processed_today: 0,
  });
  const [recentJobs, setRecentJobs] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchDashboardData = async () => {
    setLoading(true);
    try {
      const [statsRes, jobsRes] = await Promise.all([
        fetch('/api/stats'),
        fetch('/api/jobs')
      ]);

      if (statsRes.ok) {
        const statsData = await statsRes.json();
        setStats(statsData);
      }
      if (jobsRes.ok) {
        const jobsData = await jobsRes.json();
        setRecentJobs((jobsData || []).slice(0, 5));
      }
    } catch (err) {
      console.error('Failed to load dashboard data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();
  }, []);

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold text-white tracking-tight">System Dashboard</h1>
          <p className="text-slate-400 text-sm mt-1">
            Real-time status, document throughput, and storage metrics.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={fetchDashboardData}
            disabled={loading}
            className="p-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition cursor-pointer"
            title="Refresh metrics"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('upload')}
            className="px-4 py-2.5 bg-gradient-to-r from-indigo-600 to-indigo-700 hover:from-indigo-500 hover:to-indigo-600 text-white text-sm font-semibold rounded-xl shadow-lg shadow-indigo-600/25 transition flex items-center gap-2 cursor-pointer"
          >
            <UploadCloud className="w-4 h-4" />
            <span>Upload Documents</span>
          </button>
        </div>
      </div>

      {/* Metrics Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Total Documents */}
        <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl p-5 rounded-2xl relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Total Files</span>
            <div className="w-10 h-10 rounded-xl bg-indigo-500/10 text-indigo-400 flex items-center justify-center">
              <FileText className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-4">
            <span className="text-3xl font-extrabold text-white tracking-tight">{stats.total_files}</span>
            <span className="text-xs text-slate-500 ml-2">ingested</span>
          </div>
          <div className="mt-3 flex items-center text-xs text-indigo-400 font-medium">
            <span>{stats.processed_today} uploaded today</span>
          </div>
        </div>

        {/* Total Batches */}
        <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl p-5 rounded-2xl relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Batches / Jobs</span>
            <div className="w-10 h-10 rounded-xl bg-purple-500/10 text-purple-400 flex items-center justify-center">
              <Layers className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-4">
            <span className="text-3xl font-extrabold text-white tracking-tight">{stats.total_jobs}</span>
            <span className="text-xs text-slate-500 ml-2">batches</span>
          </div>
          <div className="mt-3 flex items-center text-xs text-purple-400 font-medium">
            <span>Atomic Multi-part Ingestion</span>
          </div>
        </div>

        {/* Storage Size */}
        <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl p-5 rounded-2xl relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Disk Storage</span>
            <div className="w-10 h-10 rounded-xl bg-pink-500/10 text-pink-400 flex items-center justify-center">
              <HardDrive className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-4">
            <span className="text-3xl font-extrabold text-white tracking-tight">
              {formatBytes(stats.total_bytes)}
            </span>
          </div>
          <div className="mt-3 flex items-center text-xs text-pink-400 font-medium">
            <span>Stored in ./uploads</span>
          </div>
        </div>

        {/* Users */}
        <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl p-5 rounded-2xl relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Active Users</span>
            <div className="w-10 h-10 rounded-xl bg-emerald-500/10 text-emerald-400 flex items-center justify-center">
              <Users className="w-5 h-5" />
            </div>
          </div>
          <div className="mt-4">
            <span className="text-3xl font-extrabold text-white tracking-tight">{stats.total_users}</span>
            <span className="text-xs text-slate-500 ml-2">registered</span>
          </div>
          <div className="mt-3 flex items-center text-xs text-emerald-400 font-medium">
            <span>Argon2id Hash Protected</span>
          </div>
        </div>
      </div>

      {/* System Status Banner */}
      <div className="bg-slate-900/60 border border-slate-800 p-5 rounded-2xl flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex items-center gap-3.5">
          <img src={logoImg} alt="Poneglyph" className="w-11 h-11 object-contain drop-shadow-md flex-shrink-0" />
          <div>
            <h3 className="text-sm font-bold text-white flex items-center gap-2">
              System Health & Architecture
              <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                All Systems Operational
              </span>
            </h3>
            <p className="text-xs text-slate-400 mt-0.5">
              Backend API on port 8080 • MongoDB on port 27017 • Vite Hot Reload on port 5173
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 text-xs font-mono text-slate-400">
          <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          <span>Logged in as: <strong className="text-slate-200">{user?.username}</strong> ({user?.role})</span>
        </div>
      </div>

      {/* Recent Jobs Table */}
      <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl rounded-2xl overflow-hidden shadow-xl">
        <div className="p-6 border-b border-slate-800/80 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <Clock className="w-4 h-4 text-indigo-400" />
            <h2 className="text-base font-bold text-white">Recent Ingestion Batches</h2>
          </div>
          <button
            onClick={() => onNavigate('documents')}
            className="text-xs text-indigo-400 hover:text-indigo-300 font-semibold flex items-center gap-1 transition cursor-pointer"
          >
            <span>View All Batches</span>
            <ArrowUpRight className="w-3.5 h-3.5" />
          </button>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm text-slate-300">
            <thead className="bg-slate-900/60 text-xs uppercase tracking-wider text-slate-400 font-semibold border-b border-slate-800/80">
              <tr>
                <th className="px-6 py-3.5">Batch ID</th>
                <th className="px-6 py-3.5">Uploaded Date</th>
                <th className="px-6 py-3.5">Files</th>
                <th className="px-6 py-3.5">Total Size</th>
                <th className="px-6 py-3.5 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {recentJobs.length === 0 ? (
                <tr>
                  <td colSpan="5" className="px-6 py-12 text-center text-slate-500">
                    No upload batches found. Click "Upload Documents" to ingest your first files.
                  </td>
                </tr>
              ) : (
                recentJobs.map((job) => {
                  const totalBatchSize = (job.files || []).reduce((acc, f) => acc + (f.size || 0), 0);
                  const uploadDate = new Date(job.uploaded_at).toLocaleString();
                  return (
                    <tr key={job.id} className="hover:bg-slate-800/40 transition">
                      <td className="px-6 py-4 font-mono text-xs text-indigo-300 font-medium">
                        {job.id}
                      </td>
                      <td className="px-6 py-4 text-xs text-slate-400">
                        {uploadDate}
                      </td>
                      <td className="px-6 py-4">
                        <span className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium bg-slate-800 text-slate-300 border border-slate-700/60">
                          {job.files?.length || 0} file(s)
                        </span>
                      </td>
                      <td className="px-6 py-4 font-mono text-xs text-slate-300">
                        {formatBytes(totalBatchSize)}
                      </td>
                      <td className="px-6 py-4 text-right">
                        <button
                          onClick={() => onNavigate('documents')}
                          className="text-xs px-3 py-1.5 rounded-lg bg-indigo-600/10 hover:bg-indigo-600/20 text-indigo-400 border border-indigo-500/20 transition cursor-pointer font-medium"
                        >
                          Explore
                        </button>
                      </td>
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
