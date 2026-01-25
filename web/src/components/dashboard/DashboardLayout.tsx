import { DashboardSidebar } from "./DashboardSidebar";
import { useAuth } from "@/context/AuthContext";
import { UserCircle } from "lucide-react";

export function DashboardLayout({ children }: { children: React.ReactNode }) {
    const { user } = useAuth();

    return (
        <div className="min-h-screen bg-dark-bg flex">
            <DashboardSidebar />

            <div className="flex-1 flex flex-col min-w-0 bg-[#0B101E]"> {/* Slightly different bg for content area contrast */}

                {/* Top Header */}
                <header className="h-16 flex items-center justify-between px-8 border-b border-dark-border bg-dark-bg sticky top-0 z-10">
                    <h1 className="text-xl font-semibold text-white">Dashboard</h1>

                    <div className="flex items-center gap-4">
                        <div className="flex flex-col items-end hidden sm:flex">
                            <span className="text-sm font-medium text-white">{user?.name || "User"}</span>
                            <span className="text-xs text-text-secondary uppercase">{user?.role}</span>
                        </div>
                        <div className="w-10 h-10 rounded-full bg-dark-surface border border-dark-border flex items-center justify-center text-text-secondary">
                            <UserCircle className="w-6 h-6" />
                        </div>
                    </div>
                </header>

                {/* Content */}
                <main className="flex-1 p-8 overflow-y-auto">
                    {children}
                </main>
            </div>
        </div>
    );
}
