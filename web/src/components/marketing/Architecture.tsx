import { motion } from "framer-motion";
import { Card } from "@/components/ui/Card";

export function Architecture() {
    const nodes = [
        { id: "kernel", label: "Linux Kernel", sub: "eBPF TC Hook", x: 100, y: 150, color: "bg-orange-500" },
        { id: "agent", label: "Sennet Agent", sub: "Rust (User Space)", x: 300, y: 150, color: "bg-accent" },
        { id: "cloud", label: "Control Plane", sub: "ConnectRPC (Go)", x: 500, y: 150, color: "bg-accent-alt" },
        { id: "dash", label: "Dashboard", sub: "Your Browser", x: 700, y: 150, color: "bg-success" },
    ];

    return (
        <section id="architecture" className="py-24 bg-dark-bg">
            <div className="container mx-auto px-4">
                <div className="text-center mb-16 space-y-4">
                    <h2 className="text-3xl md:text-5xl font-bold text-white">
                        Architecture that made sense
                    </h2>
                    <p className="text-text-secondary text-lg max-w-2xl mx-auto">
                        From the kernel hook to your screen in milliseconds.
                        Designed for performance, security, and simplicity.
                    </p>
                </div>

                <div className="relative max-w-5xl mx-auto h-[400px] bg-dark-surface/30 rounded-2xl border border-dashed border-dark-border p-8 flex items-center justify-center overflow-hidden">

                    {/* Connecting Line */}
                    <div className="absolute top-1/2 left-20 right-20 h-1 bg-dark-border -translate-y-1/2 z-0" />

                    {/* Animated data particles */}
                    <motion.div
                        className="absolute top-1/2 left-20 h-2 w-12 bg-accent rounded-full -translate-y-1/2 z-10 blur-sm shadow-[0_0_15px_#00D4FF]"
                        animate={{ x: [0, 800], opacity: [0, 1, 1, 0] }}
                        transition={{ duration: 2, repeat: Infinity, ease: "linear" }}
                    />
                    <motion.div
                        className="absolute top-1/2 left-20 h-2 w-12 bg-white rounded-full -translate-y-1/2 z-10"
                        animate={{ x: [0, 800], opacity: [0, 1, 1, 0] }}
                        transition={{ duration: 2, repeat: Infinity, ease: "linear", delay: 1 }}
                    />

                    {/* Nodes */}
                    <div className="relative z-20 flex w-full justify-between px-12 md:px-24">
                        {nodes.map((node, i) => (
                            <motion.div
                                key={node.id}
                                initial={{ opacity: 0, y: 20 }}
                                whileInView={{ opacity: 1, y: 0 }}
                                transition={{ delay: i * 0.2 }}
                                viewport={{ once: true }}
                                className="flex flex-col items-center"
                            >
                                <div className={`w-16 h-16 md:w-20 md:h-20 rounded-2xl ${node.color} shadow-lg flex items-center justify-center mb-4 relative group cursor-pointer`}>
                                    <div className="absolute inset-0 bg-white opacity-0 group-hover:opacity-20 transition-opacity rounded-2xl" />
                                    <span className="text-3xl font-bold text-white opacity-80">{i + 1}</span>
                                </div>
                                <Card className="p-3 md:p-4 bg-dark-surface border-dark-border/50 text-center w-40 md:w-48">
                                    <h3 className="font-bold text-white text-sm md:text-base">{node.label}</h3>
                                    <p className="text-xs text-text-secondary">{node.sub}</p>
                                </Card>
                            </motion.div>
                        ))}
                    </div>

                </div>
            </div>
        </section>
    );
}
