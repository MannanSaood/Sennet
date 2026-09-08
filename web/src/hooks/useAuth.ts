import { createContext, useContext } from 'react';
export interface SessionUser { id: string; email: string; name: string; role: string; tenant: string }
export interface AuthState {
 user: SessionUser | null; isLoading: boolean; isAuthenticated: boolean; isFirebaseEnabled: boolean;
 login: (email: string, password: string) => Promise<void>;
 loginWithGoogle: () => Promise<void>; loginWithGithub: () => Promise<void>;
 register: (email: string, password: string, name: string) => Promise<void>; logout: () => Promise<void>;
}
export const AuthContext = createContext<AuthState | undefined>(undefined);
export function useAuth() { const value = useContext(AuthContext); if (!value) throw new Error('AuthProvider required'); return value; }
