import React, { useState, useRef, useEffect } from 'react';
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
  FileSpreadsheet
} from 'lucide-react';

const MAX_FILES = 10;
const MAX_FILE_SIZE_BYTES = 25 * 1024 * 1024; // 25 MB
const ALLOWED_EXTENSIONS = ['.jpg', '.jpeg', '.png', '.webp', '.gif', '.tiff', '.tif', '.bmp', '.pdf', '.docx', '.doc'];

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
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

export default function App() {
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

      // Avoid duplicates
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
    e.stopPropagation();
    setIsDragOver(false);
    if (e.dataTransfer?.files?.length > 0) {
      handleFiles(Array.from(e.dataTransfer.files));
    }
  };

  const handleDragOver = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);
  };

  const executeUpload = async () => {
    if (selectedFiles.length === 0) {
      setErrorMessage('Please select at least 1 file to upload.');
      return;
    }

    setErrorMessage('');
    setIsUploading(true);

    const formData = new FormData();
    selectedFiles.forEach(file => {
      formData.append('files', file);
    });

    try {
      const response = await fetch('/api/upload', {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `Upload failed with HTTP ${response.status}`);
      }

      const data = await response.json();
      setUploadResult(data);
    } catch (err) {
      setErrorMessage(err.message || 'Network error occurred during batch upload.');
    } finally {
      setIsUploading(false);
    }
  };

  const copyIdToClipboard = () => {
    const idToCopy = uploadResult?.id || uploadResult?.document_id || uploadResult?.job_id;
    if (idToCopy) {
      navigator.clipboard.writeText(String(idToCopy)).then(() => {
        setCopiedId(true);
        setTimeout(() => setCopiedId(false), 2000);
      });
    }
  };

  const copyJsonToClipboard = () => {
    if (uploadResult) {
      navigator.clipboard.writeText(JSON.stringify(uploadResult, null, 2)).then(() => {
        setCopiedJson(true);
        setTimeout(() => setCopiedJson(false), 2000);
      });
    }
  };

  const resetUpload = () => {
    setSelectedFiles([]);
    setErrorMessage('');
    setUploadResult(null);
  };

  return (
    <div className="min-h-screen flex flex-col items-center justify-start py-10 px-4">
      <div className="w-full max-w-2xl flex flex-col gap-6">
        
        {/* Header */}
        <header className="text-center space-y-2">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold uppercase tracking-wider bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
            <Layers className="w-3.5 h-3.5" />
            Poneglyph Engine
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold tracking-tight bg-gradient-to-r from-white via-slate-200 to-slate-400 bg-clip-text text-transparent">
            Batch Document Ingestion
          </h1>
          <p className="text-sm text-slate-400 max-w-md mx-auto">
            Upload up to 10 readable documents or images for automated local storage and MongoDB registration.
          </p>
        </header>

        {/* Error Alert */}
        {errorMessage && (
          <div className="flex items-start gap-3 p-4 rounded-xl bg-red-500/10 border border-red-500/25 text-red-300 text-sm animate-in fade-in duration-200">
            <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5 text-red-400" />
            <div className="flex-1">{errorMessage}</div>
          </div>
        )}

        {/* Main Card */}
        {!uploadResult ? (
          <div className="bg-slate-900/75 backdrop-blur-xl border border-white/10 rounded-2xl p-6 sm:p-8 shadow-2xl shadow-indigo-950/20">
            
            {/* Dropzone */}
            <div
              onClick={() => fileInputRef.current?.click()}
              onDrop={handleDrop}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              className={`border-2 border-dashed rounded-xl p-8 text-center cursor-pointer transition-all duration-200 ${
                isDragOver 
                  ? 'border-indigo-500 bg-indigo-500/10 scale-[1.01]' 
                  : 'border-white/15 bg-slate-950/40 hover:border-indigo-500/60 hover:bg-indigo-500/5'
              }`}
            >
              <input
                ref={fileInputRef}
                type="file"
                multiple
                className="hidden"
                accept=".pdf,.png,.jpg,.jpeg,.webp,.gif,.tiff,.tif,.bmp,.docx,.doc,application/pdf,image/*,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                onChange={(e) => {
                  if (e.target.files?.length > 0) {
                    handleFiles(Array.from(e.target.files));
                  }
                  e.target.value = '';
                }}
              />

              <div className="w-14 h-14 mx-auto mb-4 rounded-full bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400">
                <UploadCloud className="w-7 h-7" />
              </div>

              <div className="text-base font-semibold text-slate-100">
                Click to browse or drag & drop files here
              </div>
              <div className="text-xs text-slate-400 mt-1">
                Images (JPEG, PNG, WebP), PDF, and Word documents (DOCX)
              </div>

              {/* Badges */}
              <div className="flex flex-wrap items-center justify-center gap-2 mt-5 text-xs text-slate-400">
                <span className="px-2.5 py-1 rounded-md bg-white/5 border border-white/10">
                  Max <strong className="text-indigo-300">10 Files</strong>
                </span>
                <span className="px-2.5 py-1 rounded-md bg-white/5 border border-white/10">
                  Max <strong className="text-indigo-300">25 MB</strong> / file
                </span>
                <span className="px-2.5 py-1 rounded-md bg-white/5 border border-white/10 flex items-center gap-1">
                  <Database className="w-3 h-3 text-emerald-400" />
                  Direct Mongo + Disk
                </span>
              </div>
            </div>

            {/* Selected Files Queue */}
            {selectedFiles.length > 0 && (
              <div className="mt-6 space-y-3 animate-in fade-in duration-200">
                <div className="flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-slate-400">
                  <span>Selected Files</span>
                  <span className="px-2 py-0.5 rounded-full bg-indigo-500/15 text-indigo-300">
                    {selectedFiles.length} / {MAX_FILES}
                  </span>
                </div>

                <div className="max-h-60 overflow-y-auto space-y-2 pr-1 custom-scrollbar">
                  {selectedFiles.map((file, idx) => {
                    const ext = getFileExtension(file.name);
                    return (
                      <div 
                        key={`${file.name}-${idx}`}
                        className="flex items-center justify-between p-3 rounded-lg bg-slate-950/50 border border-white/5 hover:border-white/15 transition-colors"
                      >
                        <div className="flex items-center gap-3 overflow-hidden">
                          <div className="w-8 h-8 rounded bg-indigo-500/10 flex items-center justify-center flex-shrink-0">
                            {getFileIcon(ext)}
                          </div>
                          <div className="flex flex-col truncate">
                            <span className="text-sm font-medium text-slate-200 truncate max-w-xs sm:max-w-md" title={file.name}>
                              {file.name}
                            </span>
                            <span className="text-xs text-slate-500">
                              {formatBytes(file.size)}
                            </span>
                          </div>
                        </div>

                        <button
                          type="button"
                          onClick={() => removeFile(idx)}
                          className="p-1.5 rounded text-slate-400 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                          title="Remove file"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>
                    );
                  })}
                </div>

                {/* Buttons */}
                <div className="flex items-center gap-3 pt-3">
                  <button
                    type="button"
                    onClick={clearAll}
                    disabled={isUploading}
                    className="px-4 py-2.5 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 font-medium text-sm transition-colors disabled:opacity-50"
                  >
                    Clear All
                  </button>

                  <button
                    type="button"
                    onClick={executeUpload}
                    disabled={isUploading || selectedFiles.length === 0}
                    className="flex-1 flex items-center justify-center gap-2 px-6 py-2.5 rounded-xl font-semibold text-sm text-white bg-gradient-to-r from-indigo-500 via-purple-500 to-pink-500 hover:shadow-lg hover:shadow-indigo-500/25 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isUploading ? (
                      <>
                        <RefreshCw className="w-4 h-4 animate-spin" />
                        <span>Uploading & Processing...</span>
                      </>
                    ) : (
                      <>
                        <UploadCloud className="w-4 h-4" />
                        <span>Upload {selectedFiles.length} {selectedFiles.length === 1 ? 'File' : 'Files'}</span>
                      </>
                    )}
                  </button>
                </div>
              </div>
            )}
          </div>
        ) : (
          /* Success Screen */
          <div className="bg-slate-900/80 backdrop-blur-xl border border-white/10 rounded-2xl p-6 sm:p-8 space-y-6 animate-in fade-in duration-300 shadow-2xl">
            <div className="flex items-center gap-3">
              <div className="w-12 h-12 rounded-full bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
                <FileCheck className="w-6 h-6" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-slate-100">Batch Ingestion Complete</h2>
                <p className="text-xs text-slate-400">Files stored in local uploads directory and recorded in MongoDB.</p>
              </div>
            </div>

            {/* ID Showcase Box */}
            <div className="flex items-center justify-between p-4 rounded-xl bg-slate-950/70 border border-indigo-500/30">
              <div className="space-y-0.5">
                <div className="text-xs uppercase font-bold tracking-wider text-slate-400">Generated ID / Primary Key</div>
                <div className="font-mono text-xl sm:text-2xl font-extrabold text-indigo-400 tracking-wide">
                  {uploadResult?.id || uploadResult?.document_id || uploadResult?.job_id || 'N/A'}
                </div>
              </div>

              <button
                type="button"
                onClick={copyIdToClipboard}
                className={`flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-semibold transition-all ${
                  copiedId 
                    ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40' 
                    : 'bg-indigo-500/15 hover:bg-indigo-500/25 text-indigo-300 border border-indigo-500/30'
                }`}
              >
                {copiedId ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                <span>{copiedId ? 'Copied!' : 'Copy ID'}</span>
              </button>
            </div>

            {/* MongoDB Payload Inspector */}
            <div className="rounded-xl bg-black/60 border border-white/10 overflow-hidden">
              <div className="flex items-center justify-between px-4 py-2.5 bg-white/5 border-b border-white/5">
                <div className="flex items-center gap-2 text-xs font-semibold text-slate-400 uppercase tracking-wider">
                  <Database className="w-3.5 h-3.5 text-emerald-400" />
                  MongoDB Record
                </div>

                <button
                  type="button"
                  onClick={copyJsonToClipboard}
                  className="text-xs text-slate-400 hover:text-white flex items-center gap-1"
                >
                  {copiedJson ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                  <span>{copiedJson ? 'Copied!' : 'Copy JSON'}</span>
                </button>
              </div>

              <pre className="p-4 text-xs font-mono text-cyan-300 overflow-x-auto max-h-64 leading-relaxed custom-scrollbar">
                {JSON.stringify(uploadResult, null, 2)}
              </pre>
            </div>

            {/* Reset Action */}
            <button
              type="button"
              onClick={resetUpload}
              className="w-full flex items-center justify-center gap-2 px-6 py-3 rounded-xl font-semibold text-sm text-white bg-indigo-600 hover:bg-indigo-500 transition-colors"
            >
              <RefreshCw className="w-4 h-4" />
              <span>Upload Another Batch</span>
            </button>
          </div>
        )}

      </div>
    </div>
  );
}
