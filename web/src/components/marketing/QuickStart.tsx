import { CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/Button";

export function QuickStart() {
    const steps = [
        {
            step: 1,
            title: "Install",
            description: "One command to rule them all. Detects your OS and architecture automatically.",
            code: "curl -sSL https://sennet.dev/install.sh | sudo bash"
        },
        {
            step: 2,
            title: "Configure",
            description: "Link the agent to your account. You'll need your API key from the dashboard.",
            code: "sudo sennet init"
        },
        {
            step: 3,
            title: "Observe",
            description: "Start the TUI or head to the web dashboard to see the magic.",
            code: "sudo sennet top"
        }
    ];

    return (
        <section className="py-24 bg-dark-surface/20">
            <div className="container mx-auto px-4">
                <div className="grid md:grid-cols-2 gap-12 items-start">

                    {/* Left Column: Text */}
                    <div className="space-y-8 sticky top-24">
                        <h2 className="text-4xl md:text-5xl font-bold leading-tight text-white">
                            Up and Running <br />
                            in <span className="text-accent underline decoration-accent/30 underline-offset-8">60 Seconds</span>
                        </h2>
                        <p className="text-text-secondary text-lg">
                            We hate complex setups as much as you do. Sennet is a single binary that requires no kernel recompilation and no reboot.
                        </p>

                        <ul className="space-y-4">
                            {["No Kernel Modules", "No Sidecars", "No Restarts", "Self-Updating"].map((item) => (
                                <li key={item} className="flex items-center gap-3 text-text-primary">
                                    <CheckCircle2 className="w-5 h-5 text-success" />
                                    {item}
                                </li>
                            ))}
                        </ul>

                        <Button size="lg" className="rounded-full px-8 mt-4">
                            View Full Installation Docs
                        </Button>
                    </div>

                    {/* Right Column: Steps */}
                    <div className="space-y-12">
                        {steps.map((step, index) => (
                            <div key={index} className="relative pl-8 md:pl-12 group">
                                {/* Connecting Line */}
                                {index !== steps.length - 1 && (
                                    <div className="absolute left-[11px] md:left-[15px] top-8 bottom-[-48px] w-0.5 bg-dark-border group-hover:bg-accent/30 transition-colors" />
                                )}

                                {/* Step Circle */}
                                <div className="absolute left-0 top-0 w-6 h-6 md:w-8 md:h-8 rounded-full bg-dark-border flex items-center justify-center text-sm font-bold text-text-secondary ring-4 ring-dark-bg group-hover:bg-accent group-hover:text-white transition-colors">
                                    {step.step}
                                </div>

                                <div className="space-y-4">
                                    <h3 className="text-2xl font-bold text-white group-hover:text-accent transition-colors">{step.title}</h3>
                                    <p className="text-text-secondary">{step.description}</p>
                                    <div className="bg-dark-surface border border-dark-border rounded-lg p-4 font-mono text-sm text-text-primary overflow-x-auto relative group-hover:border-accent/30 transition-colors shadow-lg">
                                        <span className="opacity-50 select-none mr-3">$</span>
                                        {step.code}
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>

                </div>
            </div>
        </section>
    );
}
