import React, { useState, useEffect } from 'react';
import { 
  Shield, 
  LayoutDashboard, 
  UploadCloud, 
  Files, 
  Users, 
  Settings, 
  LogOut, 
  Menu, 
  X, 
  Layers,
  ChevronRight,
  Database,
  Radio
} from 'lucide-react';

import LoginView from './components/LoginView';
import DashboardView from './components/DashboardView';
import UploadView from './components/UploadView';
import DocumentsView from './components/DocumentsView';
import UsersView from './components/UsersView';
import AdminView from './components/AdminView';
import logoImg from './assets/logo.png';

export default function App() {
  const [user, setUser] = useState(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [currentTab, setCurrentTab] = useState('dashboard');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  // Check existing session on mount
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const res = await fetch('/api/ping');
        if (res.ok) {
          const data = await res.json();
          setUser({ username: data.username, role: data.role });
          setIsAuthenticated(true);
        } else {
          setIsAuthenticated(false);
          setUser(null);
        }
      } catch (err) {
        setIsAuthenticated(false);
        setUser(null);
      } finally {
        setIsLoading(false);
      }
    };

    checkAuth();
  }, []);

  const handleLoginSuccess = (userData) => {
    setUser(userData);
    setIsAuthenticated(true);
    setCurrentTab('dashboard');
  };

  const handleLogout = async () => {
    try {
      await fetch('/api/logout', { method: 'POST' });
    } catch (e) {
      console.error('Logout error', e);
    }
    setUser(null);
    setIsAuthenticated(false);
    setCurrentTab('dashboard');
  };

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-[#0a0d14]">
        <div className="flex flex-col items-center gap-4">
          <div className="w-16 h-16 rounded-2xl bg-indigo-600/10 border border-indigo-500/30 flex items-center justify-center p-2.5 shadow-xl shadow-indigo-500/10">
            <img src={logoImg} alt="Poneglyph Logo" className="w-12 h-12 object-contain animate-pulse drop-shadow-md" />
          </div>
          <span className="text-sm font-mono text-slate-400">Loading Poneglyph...</span>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <LoginView onLoginSuccess={handleLoginSuccess} />;
  }

  const navItems = [
    { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { id: 'upload', label: 'Upload Documents', icon: UploadCloud },
    { id: 'documents', label: 'Documents & Batches', icon: Files },
    { id: 'users', label: 'Users Directory', icon: Users },
  ];

  if (user?.role === 'admin') {
    navItems.push({ id: 'admin', label: 'Admin Control', icon: Settings, adminOnly: true });
  }

  return (
    <div className="flex h-screen overflow-hidden bg-[#0a0d14] text-slate-100">
      {/* Mobile Topbar */}
      <div className="md:hidden fixed top-0 left-0 right-0 h-16 bg-[#111726]/90 border-b border-slate-800/80 backdrop-blur-xl z-30 flex items-center justify-between px-4">
        <div className="flex items-center gap-2.5">
          <img src={logoImg} alt="Poneglyph Logo" className="w-8 h-8 object-contain drop-shadow-sm" />
          <span className="font-bold text-white tracking-tight">Poneglyph</span>
        </div>
        <button
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          className="p-2 text-slate-400 hover:text-slate-200 cursor-pointer"
        >
          {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
        </button>
      </div>

      {/* Mobile Backdrop Overlay */}
      {mobileMenuOpen && (
        <div
          onClick={() => setMobileMenuOpen(false)}
          className="fixed inset-0 bg-black/60 backdrop-blur-sm z-40 md:hidden"
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed md:static inset-y-0 left-0 z-50 w-64 bg-[#0d121f] border-r border-slate-800/80 flex flex-col justify-between transition-transform duration-300 ${
          mobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
        }`}
      >
        <div>
          {/* Logo Branding */}
          <div className="p-6 border-b border-slate-800/60 flex items-center gap-3">
            <img src={logoImg} alt="Poneglyph Logo" className="w-11 h-11 object-contain drop-shadow-md flex-shrink-0" />
            <div>
              <h2 className="font-bold text-white text-base tracking-tight leading-tight">Poneglyph</h2>
              <span className="text-[10px] uppercase tracking-wider text-slate-500 font-semibold flex items-center gap-1.5 mt-0.5">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                <span>Production Ready</span>
              </span>
            </div>
          </div>

          {/* Navigation Items */}
          <nav className="p-4 space-y-1.5">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = currentTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => {
                    setCurrentTab(item.id);
                    setMobileMenuOpen(false);
                  }}
                  className={`w-full flex items-center justify-between px-3.5 py-2.5 rounded-xl text-sm font-medium transition cursor-pointer ${
                    isActive
                      ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-600/25'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <Icon className={`w-4 h-4 ${isActive ? 'text-white' : 'text-slate-400'}`} />
                    <span>{item.label}</span>
                  </div>
                  {item.adminOnly && (
                    <span className="text-[10px] bg-purple-500/20 text-purple-300 px-1.5 py-0.5 rounded font-mono font-bold">
                      ADMIN
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>

        {/* User Footer Card */}
        <div className="p-4 border-t border-slate-800/80 bg-slate-950/40">
          <div className="flex items-center justify-between p-2 rounded-xl bg-slate-900/60 border border-slate-800">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="w-8 h-8 rounded-lg bg-indigo-500/10 text-indigo-400 flex items-center justify-center font-bold text-xs uppercase border border-indigo-500/20 flex-shrink-0">
                {user?.username?.[0] || 'U'}
              </div>
              <div className="min-w-0">
                <p className="text-xs font-semibold text-white truncate">{user?.username}</p>
                <span className="text-[10px] uppercase tracking-wider text-slate-400 font-mono">
                  {user?.role}
                </span>
              </div>
            </div>
            <button
              onClick={handleLogout}
              className="p-1.5 rounded-lg text-slate-400 hover:text-red-400 hover:bg-slate-800 transition cursor-pointer"
              title="Sign Out"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 overflow-y-auto pt-20 md:pt-0 p-4 sm:p-8 lg:p-10">
        <div className="max-w-7xl mx-auto">
          {currentTab === 'dashboard' && (
            <DashboardView user={user} onNavigate={setCurrentTab} />
          )}
          {currentTab === 'upload' && (
            <UploadView onNavigate={setCurrentTab} />
          )}
          {currentTab === 'documents' && (
            <DocumentsView onNavigate={setCurrentTab} />
          )}
          {currentTab === 'users' && (
            <UsersView user={user} onNavigate={setCurrentTab} />
          )}
          {currentTab === 'admin' && user?.role === 'admin' && (
            <AdminView currentUser={user} />
          )}
        </div>
      </main>
    </div>
  );
}
