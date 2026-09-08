import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api } from '@/api/client';
import { AuthContext, type SessionUser } from '@/hooks/useAuth';
import { isFirebaseConfigured, onAuthChange, signInWithEmail, signUpWithEmail, signOut, signInWithGoogle, signInWithGithub } from '@/lib/firebase';

async function session(): Promise<SessionUser> {
 const { data } = await api.get<{tenant: string; subject: string; role: string}>('/api/session');
 return { id: data.subject, tenant: data.tenant, role: data.role, name: data.tenant, email: '' };
}
export function AuthProvider({ children }: { children: React.ReactNode }) {
 const [user, setUser] = useState<SessionUser | null>(null);
 const [isLoading, setLoading] = useState(true);
 const queries = useQueryClient();
 useEffect(() => {
  let alive = true;
  const refresh = async () => { try { const value = await session(); if (alive) setUser(value); } catch { if (alive) setUser(null); } finally { if (alive) setLoading(false); } };
  const invalid = () => { sessionStorage.removeItem('sennet_access_key'); setUser(null); queries.clear(); };
  window.addEventListener('sennet:unauthorized', invalid);
  const unsubscribe = isFirebaseConfigured ? onAuthChange(() => { void refresh(); }) : undefined;
  if (!isFirebaseConfigured) void refresh();
  return () => { alive = false; unsubscribe?.(); window.removeEventListener('sennet:unauthorized', invalid); };
 }, [queries]);
 const accept = async () => { queries.clear(); setUser(await session()); };
 return <AuthContext.Provider value={{ user, isLoading, isAuthenticated: !!user, isFirebaseEnabled: isFirebaseConfigured,
  login: async (email,password) => { await signInWithEmail(email,password); await accept(); },
  loginWithKey: async key => { sessionStorage.setItem('sennet_access_key',key.trim()); try { await accept(); } catch (error) { sessionStorage.removeItem('sennet_access_key'); throw error; } },
  loginWithGoogle: async () => { await signInWithGoogle(); await accept(); },
  loginWithGithub: async () => { await signInWithGithub(); await accept(); },
  register: async (email,password,name) => { await signUpWithEmail(email,password,name); await accept(); },
  logout: async () => { if (isFirebaseConfigured) await signOut(); sessionStorage.removeItem('sennet_access_key'); queries.clear(); setUser(null); },
 }}>{children}</AuthContext.Provider>;
}
