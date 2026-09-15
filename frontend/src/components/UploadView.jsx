import React, { useState, useRef } from 'react';
import { 
  UploadCloud, 
  FileText, 
  Image as ImageIcon, 
  FileCheck, 
  X, 
  Copy, 
  Check, 
  AlertCircle, 
  Database, 
  Layers, 
  RefreshCw, 
  HardDrive, 
  FileSpreadsheet,
  ArrowRight
} from 'lucide-react';

const MAX_FILES = 10;
const MAX_FILE_SIZE_BYTES = 25 * 1024 * 1024; // 25 MB
const ALLOWED_EXTENSIONS = ['.jpg', '.jpeg', '.png', '.webp', '.gif', '.tiff', '.tif', '.bmp', '.pdf', '.docx', '.doc'];

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

export default function UploadView({ onNavigate }) {
  const [selectedFiles, setSelectedFiles] = useState([]);
  const [errorMessage, setErrorMessage] = useState('');
  const [isUploading, setIsUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState(null);
  const [copiedId, setCopiedId] = useState(false);
  const [copiedJson, setCopiedJson] = useState(false);
  const [isDragOver, setIsDragOver] = useState(false);
  
  const fileInputRef = useRef(null);

  const handleFiles = (newFiles) => {
    setErrorMessage('');

    if (selectedFiles.length + newFiles.length > MAX_FILES) {
      setErrorMessage(`Maximum ${MAX_FILES} files allowed per batch. You have ${selectedFiles.length} and tried to add ${newFiles.length}.`);
      return;
    }

    const validNewFiles = [];

    for (const file of newFiles) {
      const ext = getFileExtension(file.name);

      if (!ALLOWED_EXTENSIONS.includes(ext)) {
        setErrorMessage(`"${file.name}" has an unsupported format. Allowed: Images, PDF, and DOCX/DOC.`);
        return;
      }

      if (file.size > MAX_FILE_SIZE_BYTES) {
        setErrorMessage(`"${file.name}" exceeds the 25 MB limit (${formatBytes(file.size)}).`);
        return;
      }

      const isDuplicate = selectedFiles.some(f => f.name === file.name && f.size === file.size);
      if (!isDuplicate) {
        validNewFiles.push(file);
      }
    }

    setSelectedFiles(prev => [...prev, ...validNewFiles]);
  };

  const removeFile = (index) => {
    setSelectedFiles(prev => prev.filter((_, i) => i !== index));
    setErrorMessage('');
  };

  const clearAll = () => {
    setSelectedFiles([]);
    setErrorMessage('');
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragOver(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      handleFiles(Array.from(e.dataTransfer.files));
    }
  };

  const handleUpload = async () => {
    if (selectedFiles.length === 0) return;

    setIsUploading(true);
    setErrorMessage('');
    setUploadResult(null);

    const formData = new FormData();
    selectedFiles.forEach((file) => {
      formData.append('files', file);
    });

    try {
      const response = await fetch('/api/upload', {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || `Upload failed with status code ${response.status}`);
      }

      const data = await response.json();
      setUploadResult(data);
      setSelectedFiles([]);
    } catch (err) {
      setErrorMessage(err.message || 'An unexpected error occurred during document upload.');
    } finally {
      setIsUploading(false);
    }
  };

  const copyToClipboard = (text, type) => {
    navigator.clipboard.writeText(text);
    if (type === 'id') {
      setCopiedId(true);
      setTimeout(() => setCopiedId(false), 2000);
    } else {
      setCopiedJson(true);
      setTimeout(() => setCopiedJson(false), 2000);
    }
  };

  const totalBytes = selectedFiles.reduce((acc, f) => acc + f.size, 0);

  return (
    <div className="space-y-8 max-w-5xl mx-auto">
      {/* View Header */}
      <div>
        <h1 className="text-2xl sm:text-3xl font-bold text-white tracking-tight">Batch Document Ingestion</h1>
        <p className="text-slate-400 text-sm mt-1">
          Upload up to 10 documents per batch. Files are validated, streamed to local storage, and cataloged in MongoDB.
        </p>
      </div>

      {/* Main Upload Dropzone Card */}
      <div className="bg-[#111726]/80 border border-slate-800/80 backdrop-blur-xl p-6 sm:p-8 rounded-2xl shadow-xl">
        {/* Dropzone */}
        <div
          onDragOver={(e) => { e.preventDefault(); setIsDragOver(true); }}
          onDragLeave={() => setIsDragOver(false)}
          onDrop={handleDrop}
          onClick={() => fileInputRef.current && fileInputRef.current.click()}
          className={`border-2 border-dashed rounded-xl p-8 sm:p-12 text-center cursor-pointer transition-all duration-200 ${
            isDragOver 
              ? 'border-indigo-500 bg-indigo-500/10 scale-[0.99]' 
              : 'border-slate-700/80 hover:border-slate-600 bg-slate-900/40 hover:bg-slate-900/60'
          }`}
        >
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            accept=".jpg,.jpeg,.png,.webp,.gif,.tiff,.tif,.bmp,.pdf,.docx,.doc"
            onChange={(e) => {
              if (e.target.files && e.target.files.length > 0) {
                handleFiles(Array.from(e.target.files));
              }
            }}
          />

          <div className="w-16 h-16 rounded-2xl bg-indigo-500/10 text-indigo-400 flex items-center justify-center mx-auto mb-4 border border-indigo-500/20">
            <UploadCloud className="w-8 h-8" />
          </div>

          <h3 className="text-base sm:text-lg font-bold text-white mb-1">
            Drag and drop your documents here
          </h3>
          <p className="text-xs sm:text-sm text-slate-400 mb-4">
            or <span className="text-indigo-400 font-semibold underline underline-offset-4">browse files</span> from your device
          </p>

          <div className="flex flex-wrap items-center justify-center gap-2 text-[11px] text-slate-500 font-mono">
            <span className="px-2 py-1 rounded bg-slate-800 border border-slate-700/60">PDF, Word (DOCX/DOC)</span>
            <span className="px-2 py-1 rounded bg-slate-800 border border-slate-700/60">Images (PNG, JPG, TIFF, WEBP)</span>
            <span className="px-2 py-1 rounded bg-slate-800 border border-slate-700/60">Max 25 MB/file</span>
            <span className="px-2 py-1 rounded bg-slate-800 border border-slate-700/60">Max 10 files/batch</span>
          </div>
        </div>

        {/* Error Notification */}
        {errorMessage && (
          <div className="mt-6 p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-start gap-3">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
            <div className="flex-1">{errorMessage}</div>
            <button 
              onClick={() => setErrorMessage('')} 
              className="text-red-400 hover:text-red-300 cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        )}

        {/* Staged Files List */}
        {selectedFiles.length > 0 && (
          <div className="mt-8">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <h4 className="text-sm font-bold text-white uppercase tracking-wider">
                  Selected Documents
                </h4>
                <span className="px-2 py-0.5 rounded-full text-xs font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
                  {selectedFiles.length} / {MAX_FILES}
                </span>
              </div>
              <div className="flex items-center gap-3 text-xs">
                <span className="text-slate-400 font-mono">Total: {formatBytes(totalBytes)}</span>
                <button
                  onClick={clearAll}
                  className="text-slate-400 hover:text-red-400 transition cursor-pointer"
                >
                  Clear all
                </button>
              </div>
            </div>

            <div className="space-y-2 max-h-60 overflow-y-auto pr-1">
              {selectedFiles.map((file, idx) => {
                const ext = getFileExtension(file.name);
                return (
                  <div
                    key={idx}
                    className="flex items-center justify-between p-3 rounded-xl bg-slate-900/60 border border-slate-800 hover:border-slate-700/80 transition"
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="p-2 rounded-lg bg-slate-800 border border-slate-700/60 flex-shrink-0">
                        {getFileIcon(ext)}
                      </div>
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-slate-200 truncate">{file.name}</p>
                        <p className="text-xs text-slate-500 font-mono">{formatBytes(file.size)}</p>
                      </div>
                    </div>
                    <button
                      onClick={() => removeFile(idx)}
                      className="p-1.5 text-slate-500 hover:text-red-400 hover:bg-slate-800 rounded-lg transition cursor-pointer"
                      title="Remove file"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  </div>
                );
              })}
            </div>

            {/* Action Buttons */}
            <div className="mt-6 flex flex-col sm:flex-row items-center justify-end gap-3 pt-6 border-t border-slate-800/80">
              <button
                onClick={clearAll}
                disabled={isUploading}
                className="w-full sm:w-auto px-5 py-2.5 rounded-xl border border-slate-700/80 text-slate-300 hover:bg-slate-800 text-sm font-medium transition cursor-pointer disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleUpload}
                disabled={isUploading || selectedFiles.length === 0}
                className="w-full sm:w-auto px-6 py-2.5 rounded-xl bg-gradient-to-r from-indigo-600 to-indigo-700 hover:from-indigo-500 hover:to-indigo-600 text-white text-sm font-semibold shadow-lg shadow-indigo-600/25 transition flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
              >
                {isUploading ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    <span>Processing Ingestion...</span>
                  </>
                ) : (
                  <>
                    <UploadCloud className="w-4 h-4" />
                    <span>Upload Batch ({selectedFiles.length})</span>
                  </>
                )}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Upload Success Card */}
      {uploadResult && (
        <div className="bg-emerald-950/20 border border-emerald-500/30 p-6 sm:p-8 rounded-2xl shadow-xl backdrop-blur-xl">
          <div className="flex items-start justify-between gap-4 mb-6">
            <div className="flex items-center gap-3">
              <div className="w-12 h-12 rounded-2xl bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center justify-center">
                <FileCheck className="w-6 h-6" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-white">Ingestion Successful</h3>
                <p className="text-xs text-slate-400">
                  Batch files written to disk and cataloged in MongoDB.
                </p>
              </div>
            </div>
            <button
              onClick={() => onNavigate('documents')}
              className="text-xs text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1.5 transition cursor-pointer bg-emerald-500/10 px-3 py-1.5 rounded-lg border border-emerald-500/20"
            >
              <span>View In Explorer</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="bg-slate-900/80 rounded-xl p-4 border border-slate-800/80 mb-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <span className="text-xs text-slate-400 uppercase tracking-wider font-semibold">
                Generated Batch ID:
              </span>
              <div className="flex items-center gap-2">
                <code className="text-xs font-mono text-indigo-400 bg-slate-950 px-2.5 py-1 rounded-md border border-slate-800">
                  {uploadResult.id}
                </code>
                <button
                  onClick={() => copyToClipboard(uploadResult.id, 'id')}
                  className="p-1.5 rounded-md hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition cursor-pointer"
                  title="Copy Batch ID"
                >
                  {copiedId ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
            </div>
          </div>

          <div className="space-y-2">
            <h5 className="text-xs uppercase font-semibold text-slate-400 tracking-wider">
              Saved Files ({uploadResult.files?.length || 0}):
            </h5>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {uploadResult.files?.map((file, idx) => (
                <div
                  key={idx}
                  className="flex items-center justify-between p-2.5 rounded-lg bg-slate-900/60 border border-slate-800 text-xs text-slate-300"
                >
                  <div className="flex items-center gap-2 truncate">
                    <FileText className="w-3.5 h-3.5 text-indigo-400 flex-shrink-0" />
                    <span className="truncate">{file.filename}</span>
                  </div>
                  <span className="font-mono text-slate-500 ml-2">{formatBytes(file.size)}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="mt-6 flex justify-end">
            <button
              onClick={() => copyToClipboard(JSON.stringify(uploadResult, null, 2), 'json')}
              className="text-xs text-slate-400 hover:text-slate-200 flex items-center gap-1.5 transition cursor-pointer"
            >
              {copiedJson ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copiedJson ? 'Copied JSON payload' : 'Copy raw JSON response'}</span>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
