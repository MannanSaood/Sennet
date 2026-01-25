import { useEffect, useState } from "react";
import { DashboardLayout } from "@/components/dashboard/DashboardLayout";
import { TrafficChart } from "@/components/dashboard/TrafficChart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Activity, AlertTriangle, CheckCircle2 } from "lucide-react";
import { api } from "@/api/client";

interface DashboardStats {
    active_agents: number;
    rx_packets: number;
    tx_packets: number;
    rx_bytes: number;
    tx_bytes: number;
    drop_count: number;
    uptime_seconds: number;
    timestamp: number;
}

export function DashboardPage() {
    const [statsData, setStatsData] = useState<DashboardStats | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        const fetchStats = async () => {
            try {
                const res = await api.get("/stats");
                setStatsData(res.data);
            } catch (err) {
                console.error("Failed to fetch dashboard stats", err);
            } finally {
                setIsLoading(false);
            }
        };

        fetchStats();
        // Poll every 10 seconds
        const interval = setInterval(fetchStats, 10000);
        return () => clearInterval(interval);
    }, []);

    // Format bytes to readable string
    const formatBytes = (bytes: number) => {
        if (!bytes) return "0 B";
        const k = 1024;
        const sizes = ["B", "KB", "MB", "GB", "TB"];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
    };

    const stats = [
        {
            title: "Active Agents",
            value: isLoading ? "..." : statsData?.active_agents.toString() || "0",
            change: "Live count",
            icon: Activity,
            trend: "neutral",
        },
        {
            title: "Packet Drops",
            value: isLoading ? "..." : statsData?.drop_count.toString() || "0",
            change: "Total dropped packets",
            icon: AlertTriangle,
            trend: "down", // keeping logic simple
        },
        {
            title: "Total Traffic (Relayed)",
            value: isLoading ? "..." : formatBytes((statsData?.rx_bytes || 0) + (statsData?.tx_bytes || 0)),
            change: "Processed volume",
            icon: CheckCircle2,
            trend: "up",
        }
    ];

    return (
        <DashboardLayout>
            <div className="space-y-8">
                {/* Stats Grid */}
                <div className="grid gap-4 md:grid-cols-3">
                    {stats.map((stat, i) => {
                        const Icon = stat.icon;
                        return (
                            <Card key={i} className="border-dark-border bg-dark-surface/50">
                                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                                    <CardTitle className="text-sm font-medium text-text-secondary">
                                        {stat.title}
                                    </CardTitle>
                                    <Icon className="h-4 w-4 text-text-secondary" />
                                </CardHeader>
                                <CardContent>
                                    <div className="text-2xl font-bold text-white">{stat.value}</div>
                                    <p className={`text-xs flex items-center mt-1 text-text-secondary`}>
                                        {stat.change}
                                    </p>
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>

                {/* Charts Section */}
                <div className="grid gap-4 md:grid-cols-1 lg:grid-cols-4">
                    <TrafficChart />

                    {/* Side Card example: Top Talkers or Recent Alerts */}
                    <Card className="col-span-1 border-dark-border bg-dark-surface/50">
                        <CardHeader>
                            <CardTitle>System Status</CardTitle>
                        </CardHeader>
                        <CardContent>
                            <div className="space-y-4">
                                <div className="flex items-center justify-between text-sm">
                                    <span className="text-text-secondary">Backend Status</span>
                                    <span className="text-success font-medium flex items-center gap-1">
                                        <div className="w-2 h-2 rounded-full bg-success animate-pulse" />
                                        Online
                                    </span>
                                </div>
                                <div className="flex items-center justify-between text-sm">
                                    <span className="text-text-secondary">Uptime</span>
                                    <span className="text-white font-mono">
                                        {isLoading ? "..." : Math.floor((statsData?.uptime_seconds || 0) / 60)} mins
                                    </span>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                </div>
            </div>
        </DashboardLayout>
    );
}
