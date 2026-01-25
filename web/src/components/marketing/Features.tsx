import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Zap, Search, Terminal, Shield, Activity, Cpu, ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/Button";

export function Features() {
    const features = [
        {
            icon: Zap,
            title: "Zero Overhead",
            description: "eBPF runs in kernel space. No sidecars, no agents eating your RAM. Sennet operates with minimal footprint.",
            color: "text-yellow-400",
            className: "md:col-span-2"
        },
        {
            icon: Search,
            title: "Packet X-Ray",
            description: "See exactly WHERE and WHY packets get dropped. Deep dive into headers without performance penalty.",
            color: "text-blue-400",
            className: ""
        },
        {
            icon: Terminal,
            title: "Beautiful CLI",
            description: "Real-time TUI that feels like htop for your network. Instant feedback, no browser required.",
            color: "text-green-400",
            className: ""
        },
        {
            icon: Shield,
            title: "K8s Native",
            description: "Pod attribution, NetworkPolicy correlation, CNI-agnostic. Understand traffic in terms of Services and Pods.",
            color: "text-purple-400",
            className: "md:col-span-2"
        },
        {
            icon: Activity,
            title: "Flow Tracking",
            description: "Per-process bandwidth tracking with PID attribution. Know exactly which process is hogging the bandwidth.",
            color: "text-red-400",
            className: ""
        },
        {
            icon: Cpu,
            title: "Edge AI",
            description: "Privacy-first anomaly detection running entirely on-device.",
            color: "text-cyan-400",
            className: ""
        }
    ];

    return (
        <section id="features" className="py-32 bg-dark-bg relative overflow-hidden">
            {/* Decorative blobs */}
            <div className="absolute top-0 right-0 w-[800px] h-[800px] bg-accent/5 rounded-full blur-[120px] pointer-events-none" />
            <div className="absolute bottom-0 left-0 w-[600px] h-[600px] bg-accent-alt/5 rounded-full blur-[120px] pointer-events-none" />

            <div className="container mx-auto px-4 relative z-10">
                <div className="grid lg:grid-cols-2 gap-12 items-end mb-20">
                    <div className="space-y-6">
                        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/5 border border-white/10 text-xs font-medium text-text-secondary uppercase tracking-wider">
                            <span className="w-2 h-2 rounded-full bg-accent animate-pulse" />
                            eBPF Powered
                        </div>
                        <h2 className="text-5xl md:text-7xl font-bold text-white tracking-tight leading-[0.9]">
                            Observability <br />
                            <span className="text-text-secondary">without the</span> <br />
                            <span className="text-transparent bg-clip-text bg-gradient-to-r from-accent to-accent-alt">sidecar tax</span>.
                        </h2>
                    </div>
                    <div className="pb-4">
                        <p className="text-xl text-text-secondary max-w-xl leading-relaxed">
                            Sennet leverages eBPF to collect deep telemetry directly from the kernel,
                            providing unparalleled visibility with negligible performance impact.
                            Stop paying for observability with your CPU cycles.
                        </p>
                        <div className="mt-8">
                            <Button variant="link" className="text-accent p-0 h-auto text-lg gap-2 group">
                                Learn about our technology <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                            </Button>
                        </div>
                    </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                    {features.map((feature, index) => {
                        const Icon = feature.icon;
                        return (
                            <Card
                                key={index}
                                variant="glass"
                                className={`group hover:-translate-y-1 transition-all duration-300 border-white/5 bg-white/[0.02] hover:bg-white/[0.05] hover:border-white/10 ${feature.className}`}
                            >
                                <CardHeader>
                                    <div className={`w-12 h-12 rounded-xl bg-white/5 flex items-center justify-center mb-4 group-hover:scale-110 transition-transform duration-500 ring-1 ring-white/10 ${feature.color}`}>
                                        <Icon className="w-6 h-6" />
                                    </div>
                                    <CardTitle className="text-xl font-semibold">{feature.title}</CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <p className="text-text-secondary leading-relaxed text-base">
                                        {feature.description}
                                    </p>
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>
            </div>
        </section>
    );
}
