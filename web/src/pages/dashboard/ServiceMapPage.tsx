import { useState } from "react";
import { DashboardLayout } from "@/components/dashboard/DashboardLayout";
import { motion, AnimatePresence } from "framer-motion";

import {
    Database,
    Globe,
    Server,
    Shield,
    X,
    ArrowRight
} from "lucide-react";

// Types
type ServiceNode = {
    id: string;
    label: string;
    type: "database" | "service" | "gateway" | "client";
    x: number;
    y: number;
    status: "healthy" | "warning" | "critical";
    throughput: string;
    errorRate: string;
    latency: string;
};

type Connection = {
    from: string;
    to: string;
    active: boolean; // Is traffic flowing?
};

export function ServiceMapPage() {
    const [selectedNode, setSelectedNode] = useState<ServiceNode | null>(null);

    const services: ServiceNode[] = [
        { id: "internet", label: "Internet", type: "client", x: 150, y: 300, status: "healthy", throughput: "N/A", errorRate: "0%", latency: "24ms" },
        { id: "lb", label: "Load Balancer", type: "gateway", x: 350, y: 300, status: "healthy", throughput: "4.2k rps", errorRate: "0.01%", latency: "2ms" },
        { id: "frontend", label: "Frontend API", type: "service", x: 550, y: 200, status: "healthy", throughput: "1.8k rps", errorRate: "0.5%", latency: "45ms" },
        { id: "auth", label: "Auth Service", type: "service", x: 550, y: 400, status: "warning", throughput: "800 rps", errorRate: "2.4%", latency: "120ms" },
        { id: "backend", label: "Backend Core", type: "service", x: 800, y: 300, status: "healthy", throughput: "2.1k rps", errorRate: "0.1%", latency: "15ms" },
        { id: "db-primary", label: "Primary DB", type: "database", x: 1000, y: 200, status: "healthy", throughput: "3k ops", errorRate: "0%", latency: "4ms" },
        { id: "db-cache", label: "Redis Cache", type: "database", x: 1000, y: 400, status: "healthy", throughput: "12k ops", errorRate: "0%", latency: "1ms" },
    ];

    const connections: Connection[] = [
        { from: "internet", to: "lb", active: true },
        { from: "lb", to: "frontend", active: true },
        { from: "lb", to: "auth", active: true },
        { from: "frontend", to: "backend", active: true },
        { from: "auth", to: "backend", active: false }, // occasional
        { from: "backend", to: "db-primary", active: true },
        { from: "backend", to: "db-cache", active: true },
    ];

    const getIcon = (type: ServiceNode["type"]) => {
        switch (type) {
            case "database": return Database;
            case "client": return Globe;
            case "gateway": return Shield;
            default: return Server;
        }
    };

    // Helper for Bézier curves
    const getPath = (start: ServiceNode, end: ServiceNode) => {
        const dist = Math.abs(end.x - start.x);
        // Control points for smooth S-curve
        const cp1x = start.x + dist * 0.5;
        const cp1y = start.y;
        const cp2x = end.x - dist * 0.5;
        const cp2y = end.y;
        return `M ${start.x} ${start.y} C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${end.x} ${end.y}`;
    };

    return (
        <DashboardLayout>
            <div className="relative h-[calc(100vh-8rem)] bg-[#0B101E] rounded-xl border border-dark-border overflow-hidden">

                {/* Header Overlay */}
                <div className="absolute top-6 left-6 z-10 pointer-events-none">
                    <h2 className="text-2xl font-bold text-white">Service Topology</h2>
                    <div className="flex items-center gap-2 mt-2">
                        <span className="flex items-center gap-1.5 text-xs text-text-secondary bg-dark-bg/50 px-2 py-1 rounded-full border border-white/5">
                            <span className="w-2 h-2 rounded-full bg-success animate-pulse" /> Live
                        </span>
                        <span className="text-xs text-text-secondary">7 Services • 98% Healthy</span>
                    </div>
                </div>

                {/* The Graph */}
                <div className="absolute inset-0">
                    {/* Background Grid */}
                    <div className="absolute inset-0 opacity-[0.03]"
                        style={{
                            backgroundImage: 'linear-gradient(#fff 1px, transparent 1px), linear-gradient(90deg, #fff 1px, transparent 1px)',
                            backgroundSize: '40px 40px'
                        }}
                    />

                    <svg className="w-full h-full pointer-events-none">
                        <defs>
                            <linearGradient id="gradient-line" x1="0%" y1="0%" x2="100%" y2="0%">
                                <stop offset="0%" stopColor="#374151" stopOpacity="0.2" />
                                <stop offset="50%" stopColor="#00D4FF" stopOpacity="0.5" />
                                <stop offset="100%" stopColor="#374151" stopOpacity="0.2" />
                            </linearGradient>
                        </defs>
                        {connections.map((conn, i) => {
                            const start = services.find(s => s.id === conn.from);
                            const end = services.find(s => s.id === conn.to);
                            if (!start || !end) return null;
                            const d = getPath(start, end);

                            return (
                                <g key={i}>
                                    {/* Base connection line */}
                                    <path d={d} fill="none" stroke="#2D3748" strokeWidth="2" />

                                    {/* Animated traffic particle */}
                                    {conn.active && (
                                        <motion.circle r="3" fill="#00D4FF">
                                            <animateMotion dur={`${1.5 + Math.random()}s`} repeatCount="indefinite" path={d}>
                                                <mpath href={`#path-${i}`} /> {/* Fallback if needed, but path prop works directly often in CSS/SMIL. Framer supports pathLength. */}
                                            </animateMotion>
                                            {/* Framer motion doesn't support 'animateMotion' directly on path string easily without custom interpolation. 
                                         Using CSS offset-path is better but support varies. 
                                         Let's use SVGs native animateMotion which is robust for this. 
                                     */}
                                            <animateMotion dur="2s" repeatCount="indefinite" path={d} rotate="auto" keyPoints="0;1" keyTimes="0;1" calcMode="linear" />
                                        </motion.circle>
                                    )}
                                    {/* Add a glow pulse on the wire */}
                                    <path d={d} fill="none" stroke="url(#gradient-line)" strokeWidth="2" strokeDasharray="10 10" className="opacity-30" />
                                </g>
                            );
                        })}
                    </svg>

                    {/* Nodes (HTML layer for interactivity) */}
                    {services.map((svc) => {
                        const Icon = getIcon(svc.type);
                        const isSelected = selectedNode?.id === svc.id;

                        return (
                            <motion.div
                                key={svc.id}
                                initial={{ scale: 0, opacity: 0 }}
                                animate={{ scale: 1, opacity: 1 }}
                                className={`absolute -ml-8 -mt-8 w-16 h-16 rounded-2xl flex items-center justify-center cursor-pointer transition-all duration-300 group ${isSelected ? "ring-2 ring-accent shadow-[0_0_30px_rgba(0,212,255,0.3)] bg-dark-surface" : "bg-dark-surface/80 hover:bg-dark-surface"
                                    }`}
                                style={{ left: svc.x, top: svc.y }}
                                onClick={() => setSelectedNode(svc)}
                            >
                                {/* Status Ring */}
                                <div className={`absolute -inset-1 rounded-2xl opacity-40 blur-sm transition-colors duration-500 ${svc.status === 'healthy' ? 'bg-success' :
                                    svc.status === 'warning' ? 'bg-warning' : 'bg-error'
                                    }`} />

                                <Icon className={`w-7 h-7 relative z-10 ${svc.status === 'healthy' ? 'text-text-primary' :
                                    svc.status === 'warning' ? 'text-warning' : 'text-error'
                                    }`} />

                                {/* Label */}
                                <div className="absolute -bottom-8 whitespace-nowrap text-xs font-semibold text-text-secondary group-hover:text-white transition-colors bg-dark-bg/90 px-2 py-0.5 rounded border border-white/5">
                                    {svc.label}
                                </div>
                            </motion.div>
                        );
                    })}
                </div>

                {/* Detail Panel */}
                <AnimatePresence>
                    {selectedNode && (
                        <motion.div
                            initial={{ x: "100%" }}
                            animate={{ x: 0 }}
                            exit={{ x: "100%" }}
                            transition={{ type: "spring", stiffness: 300, damping: 30 }}
                            className="absolute top-0 right-0 h-full w-80 bg-dark-bg/95 backdrop-blur-xl border-l border-dark-border shadow-2xl p-6"
                        >
                            <div className="flex items-center justify-between mb-8">
                                <div className="flex items-center gap-3">
                                    {(() => {
                                        const Icon = getIcon(selectedNode.type);
                                        return <div className="p-2 rounded-lg bg-white/5"><Icon className="w-5 h-5 text-accent" /></div>
                                    })()}
                                    <div>
                                        <h3 className="font-bold text-lg text-white leading-none">{selectedNode.label}</h3>
                                        <span className="text-xs text-text-secondary capitalize">{selectedNode.type}</span>
                                    </div>
                                </div>
                                <button onClick={() => setSelectedNode(null)} className="text-text-secondary hover:text-white">
                                    <X className="w-5 h-5" />
                                </button>
                            </div>

                            <div className="space-y-6">
                                <div className="grid grid-cols-2 gap-4">
                                    <div className="p-3 rounded-lg bg-white/5 border border-white/5">
                                        <span className="text-xs text-text-secondary block mb-1">Status</span>
                                        <span className={`text-sm font-bold capitalize ${selectedNode.status === 'healthy' ? 'text-success' :
                                            selectedNode.status === 'warning' ? 'text-warning' : 'text-error'
                                            }`}>
                                            {selectedNode.status}
                                        </span>
                                    </div>
                                    <div className="p-3 rounded-lg bg-white/5 border border-white/5">
                                        <span className="text-xs text-text-secondary block mb-1">Latency</span>
                                        <span className="text-sm font-bold text-white">{selectedNode.latency}</span>
                                    </div>
                                </div>

                                <div>
                                    <h4 className="text-xs uppercase tracking-wider text-text-secondary font-semibold mb-3">Metrics</h4>
                                    <div className="space-y-4">
                                        <div>
                                            <div className="flex justify-between text-sm mb-1">
                                                <span className="text-text-secondary">Throughput</span>
                                                <span className="text-white font-mono">{selectedNode.throughput}</span>
                                            </div>
                                            <div className="h-1.5 w-full bg-white/10 rounded-full overflow-hidden">
                                                <div className="h-full bg-accent w-[65%] rounded-full" />
                                            </div>
                                        </div>
                                        <div>
                                            <div className="flex justify-between text-sm mb-1">
                                                <span className="text-text-secondary">Error Rate</span>
                                                <span className="text-white font-mono">{selectedNode.errorRate}</span>
                                            </div>
                                            <div className="h-1.5 w-full bg-white/10 rounded-full overflow-hidden">
                                                <div className={`h-full w-[2%] rounded-full ${parseFloat(selectedNode.errorRate) > 1 ? 'bg-error' : 'bg-success'}`} />
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <div className="pt-6 border-t border-white/10">
                                    <h4 className="text-xs uppercase tracking-wider text-text-secondary font-semibold mb-3">Dependencies</h4>
                                    <div className="space-y-2">
                                        {connections.filter(c => c.from === selectedNode.id).map(c => (
                                            <div key={c.to} className="flex items-center gap-2 text-sm text-text-secondary p-2 rounded hover:bg-white/5">
                                                <ArrowRight className="w-4 h-4 text-text-secondary" />
                                                <span>To: <span className="text-white">{services.find(s => s.id === c.to)?.label}</span></span>
                                            </div>
                                        ))}
                                        {connections.filter(c => c.to === selectedNode.id).map(c => (
                                            <div key={c.from} className="flex items-center gap-2 text-sm text-text-secondary p-2 rounded hover:bg-white/5">
                                                <ArrowRight className="w-4 h-4 text-text-secondary rotate-180" />
                                                <span>From: <span className="text-white">{services.find(s => s.id === c.from)?.label}</span></span>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            </div>

                        </motion.div>
                    )}
                </AnimatePresence>
            </div>
        </DashboardLayout>
    );
}
