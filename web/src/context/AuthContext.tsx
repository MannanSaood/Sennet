import { createContext, useContext, useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "@/api/client";
import {
    isFirebaseConfigured,
    signInWithEmail,
    signUpWithEmail,
    signOut as firebaseSignOut,
    onAuthChange,
    getIdToken,
    signInWithGoogle,
    signInWithGithub,
} from "@/lib/firebase";
import type { User as FirebaseUser } from "@/lib/firebase";

interface User {
    id: string;
    email: string;
    name: string;
    role: "admin" | "user";
    token?: string;
}

interface AuthContextType {
    user: User | null;
    isLoading: boolean;
    login: (email: string, password: string) => Promise<void>;
    loginWithGoogle: () => Promise<void>;
    loginWithGithub: () => Promise<void>;
    register: (email: string, password: string, name: string) => Promise<void>;
    logout: () => Promise<void>;
    isAuthenticated: boolean;
    isFirebaseEnabled: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Feature Flag: Use Firebase if configured, otherwise use mock
const USE_FIREBASE = isFirebaseConfigured;
const USE_MOCK = import.meta.env.VITE_USE_MOCK === "true" && !USE_FIREBASE;

// Convert Firebase user to our User type
const firebaseUserToUser = async (firebaseUser: FirebaseUser): Promise<User> => {
    const token = await getIdToken();
    return {
        id: firebaseUser.uid,
        email: firebaseUser.email || "",
        name: firebaseUser.displayName || firebaseUser.email?.split("@")[0] || "User",
        role: "user", // Role comes from custom claims, default to user
        token: token || undefined,
    };
};

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const navigate = useNavigate();

    useEffect(() => {
        if (USE_FIREBASE) {
            // Subscribe to Firebase auth state changes
            const unsubscribe = onAuthChange(async (firebaseUser) => {
                if (firebaseUser) {
                    const userData = await firebaseUserToUser(firebaseUser);
                    setUser(userData);
                    // Set API header with Firebase token
                    if (userData.token) {
                        api.defaults.headers.Authorization = `Bearer ${userData.token}`;
                    }
                } else {
                    setUser(null);
                    delete api.defaults.headers.Authorization;
                }
                setIsLoading(false);
            });

            return () => unsubscribe();
        } else {
            // Fall back to localStorage for mock mode
            const storedUser = localStorage.getItem("sennet_user");
            if (storedUser) {
                try {
                    const parsed = JSON.parse(storedUser);
                    setUser(parsed);
                    if (parsed.token) {
                        api.defaults.headers.Authorization = `Bearer ${parsed.token}`;
                    }
                } catch (e) {
                    console.error("Failed to parse user session", e);
                    localStorage.removeItem("sennet_user");
                }
            }
            setIsLoading(false);
        }
    }, []);

    // Refresh token periodically (Firebase tokens expire after 1 hour)
    useEffect(() => {
        if (!USE_FIREBASE || !user) return;

        const refreshInterval = setInterval(async () => {
            const token = await getIdToken();
            if (token) {
                api.defaults.headers.Authorization = `Bearer ${token}`;
                setUser(prev => prev ? { ...prev, token } : null);
            }
        }, 10 * 60 * 1000); // Refresh every 10 minutes

        return () => clearInterval(refreshInterval);
    }, [user]);

    const login = async (email: string, password: string) => {
        if (USE_FIREBASE) {
            // --- FIREBASE IMPLEMENTATION ---
            try {
                const credential = await signInWithEmail(email, password);
                const userData = await firebaseUserToUser(credential.user);
                setUser(userData);
                if (userData.token) {
                    api.defaults.headers.Authorization = `Bearer ${userData.token}`;
                }
            } catch (error: any) {
                // Map Firebase error codes to user-friendly messages
                const errorCode = error.code;
                if (errorCode === "auth/user-not-found" || errorCode === "auth/wrong-password") {
                    throw new Error("Invalid email or password");
                } else if (errorCode === "auth/too-many-requests") {
                    throw new Error("Too many failed attempts. Please try again later.");
                } else {
                    throw new Error(error.message || "Login failed");
                }
            }
        } else if (USE_MOCK) {
            // --- MOCK IMPLEMENTATION ---
            return new Promise<void>((resolve, reject) => {
                setTimeout(() => {
                    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
                    if (emailRegex.test(email) && password.length >= 6) {
                        const mockUser: User = {
                            id: "mock-" + Math.random().toString(36).substring(7),
                            email,
                            name: email.split("@")[0],
                            role: "user",
                            token: "mock_token_" + Date.now()
                        };
                        setUser(mockUser);
                        localStorage.setItem("sennet_user", JSON.stringify(mockUser));
                        resolve();
                    } else {
                        reject(new Error("Invalid email format or password too short"));
                    }
                }, 1000);
            });
        } else {
            throw new Error("No authentication method configured");
        }
    };

    const loginWithGoogle = async () => {
        if (!USE_FIREBASE) {
            throw new Error("Google login requires Firebase");
        }
        try {
            const credential = await signInWithGoogle();
            const userData = await firebaseUserToUser(credential.user);
            setUser(userData);
            if (userData.token) {
                api.defaults.headers.Authorization = `Bearer ${userData.token}`;
            }
        } catch (error: any) {
            throw new Error(error.message || "Google login failed");
        }
    };

    const loginWithGithub = async () => {
        if (!USE_FIREBASE) {
            throw new Error("GitHub login requires Firebase");
        }
        try {
            const credential = await signInWithGithub();
            const userData = await firebaseUserToUser(credential.user);
            setUser(userData);
            if (userData.token) {
                api.defaults.headers.Authorization = `Bearer ${userData.token}`;
            }
        } catch (error: any) {
            throw new Error(error.message || "GitHub login failed");
        }
    };

    const register = async (email: string, password: string, name: string) => {
        if (USE_FIREBASE) {
            // --- FIREBASE IMPLEMENTATION ---
            try {
                const credential = await signUpWithEmail(email, password, name);
                const userData = await firebaseUserToUser(credential.user);
                userData.name = name; // Use provided name
                setUser(userData);
                if (userData.token) {
                    api.defaults.headers.Authorization = `Bearer ${userData.token}`;
                }
            } catch (error: any) {
                const errorCode = error.code;
                if (errorCode === "auth/email-already-in-use") {
                    throw new Error("Email already registered");
                } else if (errorCode === "auth/weak-password") {
                    throw new Error("Password should be at least 6 characters");
                } else {
                    throw new Error(error.message || "Registration failed");
                }
            }
        } else if (USE_MOCK) {
            // --- MOCK IMPLEMENTATION ---
            return new Promise<void>((resolve) => {
                setTimeout(() => {
                    const mockUser: User = {
                        id: "mock-" + Math.random().toString(36).substring(7),
                        email,
                        name,
                        role: "user",
                        token: "mock_token_" + Date.now()
                    };
                    setUser(mockUser);
                    localStorage.setItem("sennet_user", JSON.stringify(mockUser));
                    resolve();
                }, 1000);
            });
        } else {
            throw new Error("No authentication method configured");
        }
    };

    const logout = async () => {
        if (USE_FIREBASE) {
            try {
                await firebaseSignOut();
            } catch (error) {
                console.error("Logout error:", error);
            }
        }
        setUser(null);
        localStorage.removeItem("sennet_user");
        delete api.defaults.headers.Authorization;
        navigate("/");
    };

    return (
        <AuthContext.Provider value={{
            user,
            isLoading,
            login,
            loginWithGoogle,
            loginWithGithub,
            register,
            logout,
            isAuthenticated: !!user,
            isFirebaseEnabled: USE_FIREBASE
        }}>
            {children}
        </AuthContext.Provider>
    );
}

export const useAuth = () => {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error("useAuth must be used within an AuthProvider");
    }
    return context;
};
