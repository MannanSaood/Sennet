import { Link, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import {
    LayoutDashboard,
    Activity,
    Network,
    Settings,
    LogOut,
    Shield
} from "lucide-react";
import { useAuth } from "@/context/AuthContext";

const navItems = [
    { name: "Overview", href: "/dashboard", icon: LayoutDashboard },
    { name: "Live Traffic", href: "/dashboard/traffic", icon: Activity },
    { name: "Service Map", href: "/dashboard/map", icon: Network },
    { name: "Settings", href: "/dashboard/settings", icon: Settings },
];

export function DashboardSidebar() {
    const location = useLocation();
    const { logout } = useAuth();

    return (
        <div className="hidden md:flex flex-col w-64 bg-dark-bg border-r border-dark-border h-screen sticky top-0">
            {/* Header */}
            <div className="h-16 flex items-center px-6 border-b border-dark-border">
                <Link to="/" className="flex items-center gap-2 font-bold text-lg text-white group">
                    <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-gradient-to-br from-accent to-accent-alt text-white group-hover:scale-105 transition-transform">
                        <Shield className="w-5 h-5" />
                    </div>
                    Sennet console
                </Link>
            </div>

            {/* Nav */}
            <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
                {navItems.map((item) => {
                    const Icon = item.icon;
                    const isActive = location.pathname === item.href;
                    return (
                        <Link
                            key={item.name}
                            to={item.href}
                            className={cn(
                                "flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-200",
                                isActive
                                    ? "bg-accent/10 text-accent"
                                    : "text-text-secondary hover:text-white hover:bg-white/5"
                            )}
                        >
                            <Icon className="w-5 h-5" />
                            {item.name}
                            {isActive && (
                                <span className="ml-auto w-1.5 h-1.5 rounded-full bg-accent shadow-[0_0_8px_#00D4FF]" />
                            )}
                        </Link>
                    );
                })}
            </nav>

            {/* Footer */}
            <div className="p-4 border-t border-dark-border">
                <button
                    onClick={logout}
                    className="flex items-center gap-3 w-full px-3 py-2.5 rounded-lg text-sm font-medium text-text-secondary hover:text-error hover:bg-error/10 transition-colors"
                >
                    <LogOut className="w-5 h-5" />
                    Sign Out
                </button>
            </div>
        </div>
    );
}
