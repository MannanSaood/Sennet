import { useState, useEffect, useRef } from "react";

import { Terminal as TerminalIcon, Copy, Check } from "lucide-react";

type CommandStep = {
    command: string;
    output: string | string[];
    delay?: number;
};

const DEMO_STEPS: CommandStep[] = [
    {
        command: "curl -sSL https://sennet.dev/install.sh | sudo bash",
        output: [
            "Downloading sennet v0.1.3...",
            "Verifying checksums... OK",
            "Installing to /usr/local/bin/sennet... OK",
            "Setting up systemd service... OK",
            "Done! Run 'sennet init' to configure."
        ],
        delay: 1500
    },
    {
        command: "sudo sennet init",
        output: [
            "? Enter your API Key: [hidden]",
            " Authenticating... Success!",
            " Interface [eth0] auto-detected.",
            " Loading eBPF programs... OK",
            " Attaching to TC ingress/egress... OK",
            " Sennet agent is now running."
        ],
        delay: 2000
    },
    {
        command: "sudo sennet top",
        output: "", // Triggers the "UI" view
        delay: 1000
    }
];

export function TerminalDemo() {
    const [currentStep, setCurrentStep] = useState(0);
    const [typedCommand, setTypedCommand] = useState("");
    const [outputHistory, setOutputHistory] = useState<Array<{ cmd: string, out: string | string[] }>>([]);
    const [showUI, setShowUI] = useState(false);
    const [copied, setCopied] = useState(false);
    const scrollRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (showUI) return;

        let timeout: ReturnType<typeof setTimeout>;
        const step = DEMO_STEPS[currentStep];

        if (!step) {
            // Reset after done
            timeout = setTimeout(() => {
                setCurrentStep(0);
                setTypedCommand("");
                setOutputHistory([]);
                setShowUI(false);
            }, 5000);
            return () => clearTimeout(timeout);
        }

        if (typedCommand.length < step.command.length) {
            // Typing effect
            timeout = setTimeout(() => {
                setTypedCommand(step.command.substring(0, typedCommand.length + 1));
            }, 50 + Math.random() * 50);
        } else {
            // Command finished typing, show output
            timeout = setTimeout(() => {
                if (step.output === "") {
                    setShowUI(true);
                } else {
                    setOutputHistory(prev => [...prev, { cmd: step.command, out: step.output }]);
                    setTypedCommand("");
                    setCurrentStep(prev => prev + 1);
                }
            }, 500);
        }

        // Auto scroll
        if (scrollRef.current) {
            scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
        }

        return () => clearTimeout(timeout);
    }, [currentStep, typedCommand, showUI]);

    const copyInstall = () => {
        navigator.clipboard.writeText("curl -sSL https://sennet.dev/install.sh | sudo bash");
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <section className="py-24 bg-dark-bg relative">
            <div className="container mx-auto px-4 flex flex-col items-center">
                <div className="text-center mb-16 space-y-4">
                    <h2 className="text-3xl md:text-5xl font-bold text-white">
                        Runs on your infra. <br />
                        Viewer in your <span className="text-accent font-mono">terminal</span>.
                    </h2>
                    <p className="text-text-secondary text-lg">
                        No complex dashboards to learn. If you know CLI, you know Sennet.
                    </p>
                </div>

                <div className="w-full max-w-4xl relative group">
                    {/* Terminal Window */}
                    <div className="rounded-xl overflow-hidden border border-white/10 bg-[#0c0c0c] shadow-2xl font-mono text-sm md:text-base relative z-10 min-h-[500px] flex flex-col">
                        {/* Title bar */}
                        <div className="flex items-center justify-between px-4 py-3 bg-white/5 border-b border-white/5">
                            <div className="flex items-center gap-2">
                                <div className="w-3 h-3 rounded-full bg-red-500" />
                                <div className="w-3 h-3 rounded-full bg-yellow-500" />
                                <div className="w-3 h-3 rounded-full bg-green-500" />
                            </div>
                            <div className="flex items-center gap-2 text-text-secondary text-xs">
                                <TerminalIcon className="w-3 h-3" />
                                sennet — ssh — 80x24
                            </div>
                            <div className="w-12" /> {/* Spacer */}
                        </div>

                        {/* Content Area */}
                        <div ref={scrollRef} className="p-6 flex-1 overflow-y-auto font-mono custom-scrollbar">
                            {outputHistory.map((item, i) => (
                                <div key={i} className="mb-4 space-y-1">
                                    <div className="flex gap-2 text-white">
                                        <span className="text-green-500">➜</span>
                                        <span className="text-blue-500">~</span>
                                        <span>{item.cmd}</span>
                                    </div>
                                    <div className="text-text-secondary pl-4">
                                        {Array.isArray(item.out) ? (
                                            item.out.map((line, j) => <div key={j}>{line}</div>)
                                        ) : (
                                            <div>{item.out}</div>
                                        )}
                                    </div>
                                </div>
                            ))}

                            {!showUI && (
                                <div className="flex gap-2 text-white">
                                    <span className="text-green-500">➜</span>
                                    <span className="text-blue-500">~</span>
                                    <span>{typedCommand}</span>
                                    <span className="animate-pulse inline-block w-2.5 h-5 bg-white align-text-bottom ml-0.5" />
                                </div>
                            )}

                            {showUI && (
                                <div className="animate-in fade-in zoom-in duration-300">
                                    {/* Simulated TUI Interface */}
                                    <div className="grid grid-cols-12 gap-4 mb-4 text-xs md:text-sm">
                                        <div className="col-span-12 border-b border-dashed border-white/20 pb-2 mb-2 flex justify-between text-accent">
                                            <span>SENNET TOP v0.1.3 - 10.0.1.4</span>
                                            <span>UPTIME: 3d 12h 04m</span>
                                        </div>

                                        {/* Stats */}
                                        <div className="col-span-4 p-2 bg-white/5 rounded">
                                            <div className="text-text-secondary text-[10px] uppercase">RX Traffic</div>
                                            <div className="text-green-400 font-bold text-lg">1.45 Gbps</div>
                                        </div>
                                        <div className="col-span-4 p-2 bg-white/5 rounded">
                                            <div className="text-text-secondary text-[10px] uppercase">TX Traffic</div>
                                            <div className="text-blue-400 font-bold text-lg">890 Mbps</div>
                                        </div>
                                        <div className="col-span-4 p-2 bg-white/5 rounded">
                                            <div className="text-text-secondary text-[10px] uppercase">Dropped</div>
                                            <div className="text-red-400 font-bold text-lg">12 pps</div>
                                        </div>

                                        {/* Table */}
                                        <div className="col-span-12 mt-2">
                                            <div className="grid grid-cols-12 text-[10px] text-text-secondary border-b border-white/10 pb-1 mb-2 uppercase tracking-wide">
                                                <div className="col-span-1">PID</div>
                                                <div className="col-span-2">COMM</div>
                                                <div className="col-span-1">PROT</div>
                                                <div className="col-span-3">SOURCE</div>
                                                <div className="col-span-3">DESTINATION</div>
                                                <div className="col-span-2 text-right">RATE</div>
                                            </div>

                                            {[
                                                { pid: "3922", comm: "nginx", prot: "TCP", src: ":443", dst: "10.0.2.5:41233", rate: "450 Mbps" },
                                                { pid: "3922", comm: "nginx", prot: "TCP", src: ":443", dst: "10.0.2.8:58221", rate: "320 Mbps" },
                                                { pid: "4102", comm: "redis", prot: "TCP", src: ":6379", dst: "10.0.2.1:44112", rate: "120 Mbps" },
                                                { pid: "882", comm: "coredns", prot: "UDP", src: ":53", dst: "10.0.2.8:51222", rate: "50 Mbps" },
                                                { pid: "112", comm: "ssh", prot: "TCP", src: ":22", dst: "192.168.1.1:64532", rate: "Kbps" },
                                            ].map((row, i) => (
                                                <div key={i} className="grid grid-cols-12 text-white/90 py-1 border-b border-white/5 hover:bg-white/5">
                                                    <div className="col-span-1 text-accent">{row.pid}</div>
                                                    <div className="col-span-2 text-green-300">{row.comm}</div>
                                                    <div className="col-span-1 text-text-secondary">{row.prot}</div>
                                                    <div className="col-span-3">{row.src}</div>
                                                    <div className="col-span-3">{row.dst}</div>
                                                    <div className="col-span-2 text-right font-bold">{row.rate}</div>
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                    <div className="text-center pt-8">
                                        <button
                                            onClick={() => { setShowUI(false); setCurrentStep(0); setOutputHistory([]); setTypedCommand(""); }}
                                            className="text-xs text-text-secondary hover:text-white underline cursor-pointer"
                                        >
                                            Restart Demo
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>

                        {/* Quick Copy Overlay */}
                        <div className="absolute top-24 left-1/2 -translate-x-1/2 md:translate-x-0 md:left-auto md:right-8 md:top-8 z-20">
                            <button
                                onClick={copyInstall}
                                className="flex items-center gap-2 bg-accent hover:bg-accent/90 text-white px-4 py-2 rounded-full shadow-lg transition-all active:scale-95"
                            >
                                {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                                <span className="font-semibold text-sm">Copy Setup Command</span>
                            </button>
                        </div>

                    </div>

                    {/* Background Glow */}
                    <div className="absolute -inset-1 bg-gradient-to-r from-accent to-accent-alt rounded-2xl blur opacity-20 group-hover:opacity-30 transition duration-1000" />
                </div>
            </div>
        </section>
    );
}
