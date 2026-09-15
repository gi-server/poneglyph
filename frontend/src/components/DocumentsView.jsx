import React, { useState, useEffect } from 'react';
import { 
  FileText, 
  Search, 
  Download, 
  ExternalLink, 
  Copy, 
  Check, 
  Layers, 
  RefreshCw, 
  Clock, 
  ChevronDown, 
  ChevronRight,
  Image as ImageIcon,
  FileSpreadsheet,
  HardDrive
} from 'lucide-react';

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function getFileExtension(filename) {
  const idx = filename.lastIndexOf('.');
  return idx !== -1 ? filename.slice(idx).toLowerCase() : '';
}

function getFileIcon(ext) {
  if (['.jpg', '.jpeg', '.png', '.webp', '.gif', '.tiff', '.tif', '.bmp'].includes(ext)) {
    return <ImageIcon className="w-4 h-4 text-pink-400" />;
  }
  if (['.docx', '.doc'].includes(ext)) {
    return <FileSpreadsheet className="w-4 h-4 text-blue-400" />;
  }
  return <FileText className="w-4 h-4 text-indigo-400" />;
}

export default function DocumentsView({ onNavigate }) {
  const [jobs, setJobs] = useState([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [expandedJobs, setExpandedJobs] = useState({});
  const [copiedId, setCopiedId] = useState(null);

  const fetchJobs = async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/jobs');
      if (res.ok) {
        const data = await res.json();
        setJobs(data || []);
        // Automatically expand the first 2 jobs
        const initialExpanded = {};
        (data || []).slice(0, 2).forEach(j => { initialExpanded[j.id] = true; });
        setExpandedJobs(initialExpanded);
      }
    } catch (err) {
      console.error('Failed to load documents/jobs:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchJobs();
  }, []);

  const toggleExpand = (id) => {
    setExpandedJobs(prev => ({
      ...prev,
      [id]: !prev[id]
    }));
  };

  const copyJobId = (id) => {
    navigator.clipboard.writeText(id);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  // Filter jobs by ID or matching filename
  const filteredJobs = jobs.filter(job => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    const matchesId = (job.id || '').toLowerCase().includes(q);
    const matchesFile = (job.files || []).some(f => (f.filename || '').toLowerCase().includes(q));
    return matchesId || matchesFile;
  });

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold text-white tracking-tight">Documents & Batches</h1>
          <p className="text-slate-400 text-sm mt-1">
            Browse and inspect all stored document batches and download files directly from local storage.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={fetchJobs}
            disabled={loading}
            className="p-2.5 rounded-xl bg-slate-800/80 hover:bg-slate-700 text-slate-300 border border-slate-700/60 transition cursor-pointer"
            title="Refresh list"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('upload')}
            className="px-4 py-2.5 bg-gradient-to-r from-indigo-600 to-indigo-700 hover:from-indigo-500 hover:to-indigo-600 text-white text-sm font-semibold rounded-xl shadow-lg shadow-indigo-600/25 transition cursor-pointer"
          >
            Upload New Batch
          </button>
        </div>
      </div>

      {/* Search and Filters */}
      <div className="relative">
        <Search className="w-4 h-4 text-slate-500 absolute left-4 top-3.5" />
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search by batch ID or filename..."
          className="w-full pl-11 pr-4 py-3 bg-[#111726]/80 border border-slate-800 rounded-xl text-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/50 focus:border-indigo-500 transition placeholder:text-slate-500"
        />
      </div>

      {/* Jobs List */}
      <div className="space-y-4">
        {loading ? (
          <div className="p-12 text-center text-slate-500 bg-[#111726]/40 rounded-2xl border border-slate-800/80 flex items-center justify-center gap-3">
            <RefreshCw className="w-5 h-5 animate-spin text-indigo-400" />
            <span>Loading document batches from MongoDB...</span>
          </div>
        ) : filteredJobs.length === 0 ? (
          <div className="p-16 text-center bg-[#111726]/40 rounded-2xl border border-slate-800/80">
            <Layers className="w-12 h-12 text-slate-600 mx-auto mb-4" />
            <h3 className="text-base font-bold text-white mb-1">No Document Batches Found</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto mb-6">
              {searchQuery ? 'No batches or files match your search criteria.' : 'You have not uploaded any document batches yet.'}
            </p>
            <button
              onClick={() => onNavigate('upload')}
              className="px-4 py-2 bg-indigo-600 text-white text-xs font-semibold rounded-lg hover:bg-indigo-500 transition cursor-pointer"
            >
              Upload First Batch
            </button>
          </div>
        ) : (
          filteredJobs.map((job) => {
            const isExpanded = !!expandedJobs[job.id];
            const totalSize = (job.files || []).reduce((acc, f) => acc + (f.size || 0), 0);
            const dateStr = new Date(job.uploaded_at).toLocaleString();

            return (
              <div
                key={job.id}
                className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl rounded-2xl overflow-hidden shadow-lg transition"
              >
                {/* Batch Header Bar */}
                <div
                  onClick={() => toggleExpand(job.id)}
                  className="p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 cursor-pointer hover:bg-slate-800/40 transition select-none"
                >
                  <div className="flex items-center gap-3.5">
                    <button className="text-slate-400">
                      {isExpanded ? <ChevronDown className="w-5 h-5" /> : <ChevronRight className="w-5 h-5" />}
                    </button>
                    <div className="w-10 h-10 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 flex items-center justify-center flex-shrink-0">
                      <Layers className="w-5 h-5" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-xs font-semibold text-indigo-300">
                          Batch: {job.id}
                        </span>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            copyJobId(job.id);
                          }}
                          className="p-1 text-slate-500 hover:text-slate-300 rounded transition"
                          title="Copy ID"
                        >
                          {copiedId === job.id ? (
                            <Check className="w-3.5 h-3.5 text-emerald-400" />
                          ) : (
                            <Copy className="w-3.5 h-3.5" />
                          )}
                        </button>
                      </div>
                      <div className="flex items-center gap-3 text-xs text-slate-500 mt-0.5">
                        <span className="flex items-center gap-1">
                          <Clock className="w-3 h-3" />
                          {dateStr}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-4 text-xs font-mono self-end sm:self-auto">
                    <span className="px-2.5 py-1 rounded-md bg-slate-800 text-slate-300 border border-slate-700/60 font-sans font-medium">
                      {job.files?.length || 0} file(s)
                    </span>
                    <span className="text-slate-400">
                      {formatBytes(totalSize)}
                    </span>
                  </div>
                </div>

                {/* Expanded Files View */}
                {isExpanded && (
                  <div className="p-5 pt-0 border-t border-slate-800/60 bg-slate-950/40">
                    <div className="mt-4 space-y-2">
                      <h5 className="text-[11px] font-bold uppercase tracking-wider text-slate-400 mb-2">
                        Cataloged Files
                      </h5>
                      {job.files?.map((file, fIdx) => {
                        const ext = getFileExtension(file.filename);
                        const downloadUrl = `/api/jobs/${job.id}/files/${encodeURIComponent(file.filename)}`;

                        return (
                          <div
                            key={fIdx}
                            className="flex items-center justify-between p-3 rounded-xl bg-slate-900/80 border border-slate-800 hover:border-slate-700 transition"
                          >
                            <div className="flex items-center gap-3 min-w-0">
                              <div className="p-2 rounded-lg bg-slate-800 border border-slate-700/60 flex-shrink-0">
                                {getFileIcon(ext)}
                              </div>
                              <div className="min-w-0">
                                <p className="text-sm font-medium text-slate-200 truncate">{file.filename}</p>
                                <div className="flex items-center gap-2 text-xs text-slate-500 font-mono">
                                  <span>{formatBytes(file.size)}</span>
                                  <span>•</span>
                                  <span className="text-[11px] text-slate-600">{file.mime_type || 'binary/octet-stream'}</span>
                                </div>
                              </div>
                            </div>

                            <div className="flex items-center gap-2 flex-shrink-0">
                              <a
                                href={downloadUrl}
                                target="_blank"
                                rel="noreferrer"
                                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700/60 text-xs flex items-center gap-1.5 transition cursor-pointer"
                                title="Open or Download file"
                              >
                                <ExternalLink className="w-3.5 h-3.5" />
                                <span className="hidden sm:inline">View / Download</span>
                              </a>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
