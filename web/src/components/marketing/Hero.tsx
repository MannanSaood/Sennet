import { Button } from "@/components/ui/Button";
import { Link } from "react-router-dom";
import { ChevronRight, Terminal } from "lucide-react";
import { NetworkAnimation } from "./NetworkAnimation";
import { motion } from "framer-motion";

export function Hero() {
    return (
        <div className="relative min-h-screen pt-16 flex items-center justify-center overflow-hidden bg-dark-bg selection:bg-accent/20">
            {/* Dynamic Background Animation (Canvas) */}
            <NetworkAnimation />

            {/* Grid Pattern Overlay */}
            <div
                className="absolute inset-0 z-0 opacity-[0.03] pointer-events-none"
                style={{
                    backgroundImage: `linear-gradient(to right, #808080 1px, transparent 1px), linear-gradient(to bottom, #808080 1px, transparent 1px)`,
                    backgroundSize: '4rem 4rem',
                    maskImage: 'radial-gradient(ellipse at center, black 40%, transparent 80%)'
                }}
            />

            {/* Ambient Gradient Orbs - Animated */}
            <motion.div
                animate={{
                    scale: [1, 1.2, 1],
                    opacity: [0.1, 0.15, 0.1],
                    x: [0, 50, 0],
                    y: [0, -30, 0]
                }}
                transition={{ duration: 15, repeat: Infinity, ease: "easeInOut" }}
                className="absolute top-0 right-0 w-[800px] h-[800px] bg-accent/10 rounded-full blur-[120px] pointer-events-none mix-blend-screen"
            />
            <motion.div
                animate={{
                    scale: [1, 1.3, 1],
                    opacity: [0.05, 0.1, 0.05],
                    x: [0, -40, 0],
                    y: [0, 40, 0]
                }}
                transition={{ duration: 18, repeat: Infinity, ease: "easeInOut", delay: 2 }}
                className="absolute bottom-0 left-0 w-[600px] h-[600px] bg-accent-alt/10 rounded-full blur-[120px] pointer-events-none mix-blend-screen"
            />

            <div className="container mx-auto px-4 z-10 grid gap-16 text-center md:text-left md:grid-cols-2 items-center">

                {/* Text Content */}
                <div className="space-y-8 relative">
                    <motion.div
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.5 }}
                        className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-accent/5 text-accent text-sm font-medium border border-accent/20 backdrop-blur-sm"
                    >
                        <span className="relative flex h-2 w-2">
                            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent opacity-75"></span>
                            <span className="relative inline-flex rounded-full h-2 w-2 bg-accent"></span>
                        </span>
                        <span className="tracking-wide text-xs uppercase font-bold opacity-90">v0.1.3 Released</span>
                    </motion.div>

                    <motion.h1
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.5, delay: 0.1 }}
                        className="text-5xl md:text-7xl font-bold tracking-tight text-white leading-[0.95]"
                    >
                        X-Ray Vision <br />
                        <span className="text-transparent bg-clip-text bg-gradient-to-r from-white via-white/50 to-white/20">
                            for Your Network
                        </span>
                    </motion.h1>

                    <motion.p
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.5, delay: 0.2 }}
                        className="text-lg md:text-xl text-text-secondary max-w-lg mx-auto md:mx-0 leading-relaxed"
                    >
                        eBPF-powered observability that drops in and Just Works.
                        See every packet, trace every drop, and understand your traffic at kernel speed.
                    </motion.p>

                    <motion.div
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.5, delay: 0.3 }}
                        className="flex flex-col sm:flex-row gap-4 justify-center md:justify-start pt-4"
                    >
                        <Link to="/register">
                            <Button size="lg" className="text-base h-12 px-8 rounded-full shadow-[0_0_30px_rgba(0,212,255,0.25)] hover:shadow-[0_0_40px_rgba(0,212,255,0.4)] hover:scale-105 transition-all duration-300">
                                Get Started Free
                                <ChevronRight className="ml-2 w-4 h-4" />
                            </Button>
                        </Link>
                        <Link to="/docs">
                            <Button size="lg" variant="secondary" className="text-base h-12 px-8 rounded-full border-white/5 bg-white/5 hover:bg-white/10 backdrop-blur-md">
                                Documentation
                            </Button>
                        </Link>
                    </motion.div>
                </div>

                {/* Hero Visual / Terminal Preview */}
                <motion.div
                    initial={{ opacity: 0, scale: 0.9, rotateX: 10 }}
                    animate={{ opacity: 1, scale: 1, rotateX: 0 }}
                    transition={{ duration: 0.8, delay: 0.4, type: "spring" }}
                    className="relative mx-auto w-full max-w-lg lg:max-w-xl perspective-1000 group"
                >
                    <div className="relative rounded-xl border border-white/10 bg-[#0c0c0c]/90 backdrop-blur-xl shadow-2xl overflow-hidden transform rotate-y-[-5deg] rotate-x-[5deg] group-hover:rotate-0 transition-transform duration-700 ease-out z-10">
                        {/* Terminal Tab Bar */}
                        <div className="flex items-center gap-2 px-4 py-3 border-b border-white/5 bg-white/5">
                            <div className="flex gap-2">
                                <div className="w-3 h-3 rounded-full bg-[#FF5F56] border border-[#E0443E]" />
                                <div className="w-3 h-3 rounded-full bg-[#FFBD2E] border border-[#DEA123]" />
                                <div className="w-3 h-3 rounded-full bg-[#27C93F] border border-[#1AAB29]" />
                            </div>
                            <div className="ml-4 flex items-center gap-2 text-xs text-text-secondary font-mono opacity-50">
                                <Terminal className="w-3 h-3" />
                                sennet — top — 80x24
                            </div>
                        </div>

                        {/* Terminal Content */}
                        <div className="p-6 font-mono text-sm leading-relaxed text-text-secondary h-[320px] overflow-hidden">
                            <div className="flex items-center gap-2 text-text-primary mb-4">
                                <span className="text-success">➜</span>
                                <span>~</span>
                                <span className="text-accent typing-cursor">sudo sennet top</span>
                            </div>

                            <div className="space-y-1">
                                <div className="flex justify-between text-[10px] uppercase tracking-wider opacity-40 border-b border-white/10 pb-2 mb-2 font-bold">
                                    <span>Prot</span> <span>Source</span> <span>Dest</span> <span>RX/TX</span> <span>Drops</span>
                                </div>

                                {[
                                    { p: "TCP", s: "10.0.1.4:443", d: "10.0.2.8:34211", rx: "1.2Gb", drop: "0" },
                                    { p: "UDP", s: "10.0.1.2:53", d: "10.0.2.8:58221", rx: "420Kb", drop: "0" },
                                    { p: "TCP", s: "10.0.1.4:80", d: "10.0.2.9:44112", rx: "850Mb", drop: "12" }, // Drop!
                                    { p: "ICMP", s: "192.168.1.1", d: "10.0.2.8", rx: "64b", drop: "0" },
                                    { p: "TCP", s: "10.0.1.5:8080", d: "10.0.2.4:1223", rx: "12Mb", drop: "0" },
                                ].map((row, i) => (
                                    <div key={i} className={`flex justify-between items-center py-1 border-b border-white/5 transition-colors hover:bg-white/5 ${row.drop !== "0" ? "text-error" : ""}`}>
                                        <span className="w-10 text-xs opacity-70">{row.p}</span>
                                        <span className="w-32 truncate">{row.s}</span>
                                        <span className="w-32 truncate">{row.d}</span>
                                        <span className="w-16 text-right font-medium text-text-primary">{row.rx}</span>
                                        <span className={`w-12 text-right ${row.drop !== "0" ? "font-bold bg-error/10 px-1 rounded text-error" : "opacity-30"}`}>{row.drop}</span>
                                    </div>
                                ))}

                                <div className="mt-8 pt-4 border-t border-white/10 text-xs text-text-secondary flex justify-between font-mono opacity-60">
                                    <span>Processing: 1.4M pps</span>
                                    <span>Load: 0.4%</span>
                                    <span className="text-success">● Live</span>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Reflection / Glow below */}
                    <div className="absolute top-full left-4 right-4 h-8 bg-accent/20 blur-xl opacity-30 transform scale-x-90" />
                </motion.div>
            </div>
        </div>
    );
}
