import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { Activity, ArrowRight } from 'lucide-react';

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(action: () => Promise<void>) {
    setBusy(true);
    setError('');
    try {
      await action();
      navigate('/dashboard');
    } catch {
      setError('Sign-in failed. Check your account and server connection.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login-shell">
      <Link to="/" className="brand"><Activity /> sennet<span> / observability</span></Link>
      <main className="login-panel">
        <p className="eyebrow">YOUR SYSTEMS, IN CONTEXT</p>
        <h1>Open your workspace.</h1>
        <p className="muted">Sign in with your account. Sennet does not require or issue user API keys.</p>
        {auth.isFirebaseEnabled ? <>
          <form onSubmit={(event) => { event.preventDefault(); void submit(() => auth.login(email, password)); }}>
            <label htmlFor="email">Email</label>
            <input id="email" type="email" required value={email} onChange={(event) => setEmail(event.target.value)} />
            <label htmlFor="password">Password</label>
            <input id="password" type="password" required value={password} onChange={(event) => setPassword(event.target.value)} />
            <button className="primary" disabled={busy}>Sign in <ArrowRight size={16} /></button>
          </form>
          <div className="divider">or</div>
          <button disabled={busy} onClick={() => void submit(auth.loginWithGoogle)}>Continue with Google</button>
          <button disabled={busy} onClick={() => void submit(auth.loginWithGithub)}>Continue with GitHub</button>
          <Link to="/register">Create account</Link>
        </> : <p role="alert" className="error-banner">Login is not configured for this deployment.</p>}
        {error && <p role="alert" className="error-banner">{error}</p>}
        <Link to="/docs">Connection and installation guide →</Link>
      </main>
    </div>
  );
}
