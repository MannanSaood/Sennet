// Firebase configuration for Sennet
// This file initializes Firebase Auth for the frontend

import { initializeApp } from 'firebase/app';
import type { FirebaseApp } from 'firebase/app';
import {
    getAuth,
    signInWithEmailAndPassword,
    createUserWithEmailAndPassword,
    signOut as firebaseSignOut,
    onAuthStateChanged,
    sendPasswordResetEmail,
    updateProfile,
    GoogleAuthProvider,
    signInWithPopup,
    GithubAuthProvider
} from 'firebase/auth';
import type { Auth, User } from 'firebase/auth';

// Firebase configuration from environment variables
const firebaseConfig = {
    apiKey: import.meta.env.VITE_FIREBASE_API_KEY,
    authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN,
    projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID,
    storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET,
    messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID,
    appId: import.meta.env.VITE_FIREBASE_APP_ID,
};

// Validate config
const validateConfig = () => {
    const required = ['apiKey', 'authDomain', 'projectId'];
    for (const key of required) {
        if (!firebaseConfig[key as keyof typeof firebaseConfig]) {
            console.warn(`Missing Firebase config: ${key}. Using mock auth.`);
            return false;
        }
    }
    return true;
};

// Initialize Firebase
let app: FirebaseApp | null = null;
let auth: Auth | null = null;

export const isFirebaseConfigured = validateConfig();

if (isFirebaseConfigured) {
    app = initializeApp(firebaseConfig);
    auth = getAuth(app);
}

// Auth helper functions
export const signInWithEmail = async (email: string, password: string) => {
    if (!auth) throw new Error('Firebase not configured');
    return signInWithEmailAndPassword(auth, email, password);
};

export const signUpWithEmail = async (email: string, password: string, displayName: string) => {
    if (!auth) throw new Error('Firebase not configured');
    const credential = await createUserWithEmailAndPassword(auth, email, password);
    if (credential.user) {
        await updateProfile(credential.user, { displayName });
    }
    return credential;
};

export const signOut = async () => {
    if (!auth) throw new Error('Firebase not configured');
    return firebaseSignOut(auth);
};

export const resetPassword = async (email: string) => {
    if (!auth) throw new Error('Firebase not configured');
    return sendPasswordResetEmail(auth, email);
};

export const signInWithGoogle = async () => {
    if (!auth) throw new Error('Firebase not configured');
    const provider = new GoogleAuthProvider();
    return signInWithPopup(auth, provider);
};

export const signInWithGithub = async () => {
    if (!auth) throw new Error('Firebase not configured');
    const provider = new GithubAuthProvider();
    return signInWithPopup(auth, provider);
};

// Get current user's ID token for API calls
export const getIdToken = async (): Promise<string | null> => {
    if (!auth?.currentUser) return null;
    return auth.currentUser.getIdToken();
};

// Force refresh token (call after role changes)
export const refreshIdToken = async (): Promise<string | null> => {
    if (!auth?.currentUser) return null;
    return auth.currentUser.getIdToken(true);
};

// Subscribe to auth state changes
export const onAuthChange = (callback: (user: User | null) => void) => {
    if (!auth) {
        console.warn('Firebase not configured, auth state will not change');
        return () => { };
    }
    return onAuthStateChanged(auth, callback);
};

export { auth, app };
export type { User };
