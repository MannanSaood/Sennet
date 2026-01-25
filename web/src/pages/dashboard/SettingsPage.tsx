import { useState, useEffect } from "react";
import { DashboardLayout } from "@/components/dashboard/DashboardLayout";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useAuth } from "@/context/AuthContext";
import { api } from "@/api/client";
import {
    Key,
    User,
    Bell,
    Shield,
    CreditCard,
    LogOut,
    Copy,
    Check,
    Trash2,
    Globe
} from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";

type Tab = "general" | "security" | "notifications" | "team" | "billing";

interface APIKey {
    Key: string;
    Name: string;
    CreatedAt: string;
}

export function SettingsPage() {
    const { user, logout } = useAuth();
    const [activeTab, setActiveTab] = useState<Tab>("general");
    const [copiedKey, setCopiedKey] = useState<string | null>(null);
    const [keys, setKeys] = useState<APIKey[]>([]);
    const [isLoading, setIsLoading] = useState(false);

    useEffect(() => {
        if (activeTab === "security") {
            fetchKeys();
        }
    }, [activeTab]);

    const fetchKeys = async () => {
        setIsLoading(true);
        try {
            const res = await api.get("/api/keys");
            setKeys(res.data || []);
        } catch (err) {
            console.error("Failed to fetch keys", err);
        } finally {
            setIsLoading(false);
        }
    };

    const handleGenerateKey = async () => {
        try {
            const name = `Key ${keys.length + 1}`;
            await api.post("/api/keys/create", { name });
            fetchKeys();
        } catch (err) {
            console.error("Failed to create key", err);
        }
    };

    const copyKey = (key: string) => {
        navigator.clipboard.writeText(key);
        setCopiedKey(key);
        setTimeout(() => setCopiedKey(null), 2000);
    };

    const navItems = [
        { id: "general", label: "General", icon: User },
        { id: "security", label: "Security & API", icon: Key },
        { id: "notifications", label: "Notifications", icon: Bell },
        { id: "team", label: "Team Members", icon: Shield },
        { id: "billing", label: "Billing", icon: CreditCard },
    ];

    return (
        <DashboardLayout>
            <div className="max-w-6xl mx-auto space-y-6">
                <div>
                    <h2 className="text-3xl font-bold text-white tracking-tight">Account Settings</h2>
                    <p className="text-text-secondary mt-1">Manage your profile, preferences, and API credentials.</p>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
                    {/* Sidebar Nav */}
                    <aside className="space-y-1">
                        {navItems.map((item) => {
                            const Icon = item.icon;
                            const isActive = activeTab === item.id;
                            return (
                                <button
                                    key={item.id}
                                    onClick={() => setActiveTab(item.id as Tab)}
                                    className={`w-full flex items-center gap-3 px-4 py-3 text-sm font-medium rounded-lg transition-all duration-200 ${isActive
                                        ? "bg-accent/10 text-accent ring-1 ring-accent/20"
                                        : "text-text-secondary hover:text-white hover:bg-white/5"
                                        }`}
                                >
                                    <Icon className="w-4 h-4" />
                                    {item.label}
                                </button>
                            );
                        })}

                        <div className="pt-4 mt-4 border-t border-dark-border">
                            <button
                                onClick={logout}
                                className="w-full flex items-center gap-3 px-4 py-3 text-sm font-medium text-error/80 hover:text-error hover:bg-error/10 rounded-lg transition-colors"
                            >
                                <LogOut className="w-4 h-4" />
                                Sign Out
                            </button>
                        </div>
                    </aside>

                    {/* Main Content Area */}
                    <div className="md:col-span-3 space-y-6">
                        <AnimatePresence mode="wait">
                            <motion.div
                                key={activeTab}
                                initial={{ opacity: 0, x: 20 }}
                                animate={{ opacity: 1, x: 0 }}
                                exit={{ opacity: 0, x: -20 }}
                                transition={{ duration: 0.2 }}
                            >
                                {activeTab === "general" && (
                                    <div className="space-y-6">
                                        <section className="p-6 rounded-xl border border-dark-border bg-dark-bg/50 backdrop-blur-xl">
                                            <h3 className="text-lg font-medium text-white mb-6">Profile Information</h3>

                                            <div className="flex items-center gap-6 mb-8">
                                                <div className="w-20 h-20 rounded-full bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center text-white text-2xl font-bold border-4 border-dark-bg shadow-xl">
                                                    {user?.name?.[0] || "U"}
                                                </div>
                                                <div className="space-y-2">
                                                    <Button variant="secondary" size="sm">Change Avatar</Button>
                                                    <p className="text-xs text-text-secondary">JPG, GIF or PNG. 1MB max.</p>
                                                </div>
                                            </div>

                                            <div className="grid gap-6 md:grid-cols-2">
                                                <Input label="Full Name" defaultValue={user?.name} className="bg-dark-bg/50" />
                                                <Input label="Email Address" defaultValue={user?.email} disabled className="bg-dark-bg/50 opacity-60" />
                                                <div className="md:col-span-2">
                                                    <Input label="Bio" placeholder="Tell us a little about yourself" className="bg-dark-bg/50" />
                                                </div>
                                            </div>

                                            <div className="mt-8 flex justify-end">
                                                <Button>Save Changes</Button>
                                            </div>
                                        </section>
                                    </div>
                                )}

                                {activeTab === "security" && (
                                    <div className="space-y-6">
                                        <section className="p-6 rounded-xl border border-dark-border bg-dark-bg/50 backdrop-blur-xl">
                                            <div className="flex items-center justify-between mb-6">
                                                <h3 className="text-lg font-medium text-white">API Keys</h3>
                                                <Button size="sm" className="gap-2" onClick={handleGenerateKey}>
                                                    <Key className="w-3 h-3" /> Generate New Key
                                                </Button>
                                            </div>

                                            <div className="space-y-4">
                                                {isLoading && <p className="text-text-secondary text-sm">Loading keys...</p>}
                                                {!isLoading && keys.length === 0 && <p className="text-text-secondary text-sm">No API keys found.</p>}
                                                {keys.map((k) => (
                                                    <div key={k.Key} className="p-4 rounded-lg border border-dark-border bg-dark-bg/30">
                                                        <div className="flex items-center justify-between mb-2">
                                                            <span className="text-sm font-medium text-white flex items-center gap-2">
                                                                <Globe className="w-3 h-3 text-success" /> {k.Name}
                                                            </span>
                                                            <span className="text-xs text-text-secondary">Created: {new Date(k.CreatedAt).toLocaleDateString()}</span>
                                                        </div>
                                                        <div className="flex gap-2">
                                                            <div className="flex-1 font-mono text-xs bg-black/50 border border-white/5 rounded px-3 py-2 text-text-secondary flex items-center justify-between group">
                                                                <span>{k.Key}</span>
                                                            </div>
                                                            <div className="flex gap-2">
                                                                <Button variant="secondary" size="sm" onClick={() => copyKey(k.Key)} className="w-10 px-0">
                                                                    {copiedKey === k.Key ? <Check className="w-4 h-4 text-success" /> : <Copy className="w-4 h-4" />}
                                                                </Button>
                                                                <Button variant="danger" size="sm" className="w-10 px-0 hover:bg-error/20 bg-transparent border border-white/5 text-error">
                                                                    <Trash2 className="w-4 h-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                        <p className="mt-2 text-xs text-text-secondary">
                                                            This key has full access to your organization's data. Keep it secret.
                                                        </p>
                                                    </div>
                                                ))}
                                            </div>
                                        </section>

                                        <section className="p-6 rounded-xl border border-error/20 bg-error/5">
                                            <h3 className="text-lg font-medium text-error mb-2">Danger Zone</h3>
                                            <p className="text-sm text-text-secondary mb-6">Irreversible actions that affect your account data.</p>
                                            <Button variant="danger" className="bg-error hover:bg-error/90 text-white border-none">Delete Account</Button>
                                        </section>
                                    </div>
                                )}

                                {["notifications", "team", "billing"].includes(activeTab) && (
                                    <div className="p-12 text-center border border-dashed border-dark-border rounded-xl">
                                        <div className="mx-auto w-12 h-12 rounded-full bg-white/5 flex items-center justify-center mb-4">
                                            <CreditCard className="w-6 h-6 text-text-secondary" />
                                        </div>
                                        <h3 className="text-lg font-medium text-white mb-2">Coming Soon</h3>
                                        <p className="text-text-secondary max-w-sm mx-auto">
                                            This section is currently under development. Check back later for updates.
                                        </p>
                                    </div>
                                )}
                            </motion.div>
                        </AnimatePresence>
                    </div>
                </div>
            </div>
        </DashboardLayout>
    );
}
