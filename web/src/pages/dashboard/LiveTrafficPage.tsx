import { DashboardLayout } from "@/components/dashboard/DashboardLayout";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";


export function LiveTrafficPage() {
    const flows = [
        { src: "frontend-svc", dest: "api-gateway", protocol: "TCP", bytes: "1.2 GB", packets: "14.5k", risk: "low" },
        { src: "api-gateway", dest: "auth-service", protocol: "gRPC", bytes: "450 MB", packets: "8.2k", risk: "low" },
        { src: "api-gateway", dest: "payment-svc", protocol: "gRPC", bytes: "120 MB", packets: "2.1k", risk: "low" },
        { src: "payment-svc", dest: "stripe-api", protocol: "HTTPS", bytes: "50 MB", packets: "800", risk: "high" }, // External
        { src: "worker-node-1", dest: "postgres-db", protocol: "TCP", bytes: "5.8 GB", packets: "890k", risk: "low" },
        { src: "unknown-ip", dest: "frontend-svc", protocol: "TCP", bytes: "12 KB", packets: "140", risk: "critical" },
    ];

    return (
        <DashboardLayout>
            <div className="space-y-6">
                <div>
                    <h2 className="text-3xl font-bold text-white">Live Traffic</h2>
                    <p className="text-text-secondary">Real-time flow analysis from eBPF probes</p>
                </div>

                <Card className="border-dark-border bg-dark-surface/50">
                    <CardHeader>
                        <CardTitle>Active Flows</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="relative overflow-x-auto">
                            <table className="w-full text-sm text-left text-text-secondary">
                                <thead className="text-xs text-text-primary uppercase bg-white/5">
                                    <tr>
                                        <th scope="col" className="px-6 py-3 rounded-l-lg">Source</th>
                                        <th scope="col" className="px-6 py-3">Destination</th>
                                        <th scope="col" className="px-6 py-3">Protocol</th>
                                        <th scope="col" className="px-6 py-3">Bytes</th>
                                        <th scope="col" className="px-6 py-3">Packets</th>
                                        <th scope="col" className="px-6 py-3 rounded-r-lg">Risk</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {flows.map((flow, i) => (
                                        <tr key={i} className="border-b border-dark-border/50 hover:bg-white/5 transition-colors">
                                            <td className="px-6 py-4 font-medium text-white">{flow.src}</td>
                                            <td className="px-6 py-4">{flow.dest}</td>
                                            <td className="px-6 py-4">
                                                <span className="px-2 py-1 rounded-md bg-white/10 text-xs font-mono">{flow.protocol}</span>
                                            </td>
                                            <td className="px-6 py-4">{flow.bytes}</td>
                                            <td className="px-6 py-4">{flow.packets}</td>
                                            <td className="px-6 py-4">
                                                <span className={`px-2 py-1 rounded-full text-xs font-bold ${flow.risk === "critical" ? "bg-error/20 text-error" :
                                                    flow.risk === "high" ? "bg-warning/20 text-warning" :
                                                        "bg-success/20 text-success"
                                                    }`}>
                                                    {flow.risk.toUpperCase()}
                                                </span>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </DashboardLayout>
    );
}
