import { Navbar } from "@/components/layout/Navbar";
import { Footer } from "@/components/layout/Footer";
import { Hero } from "@/components/marketing/Hero";
import { Features } from "@/components/marketing/Features";
import { TerminalDemo } from "@/components/marketing/TerminalDemo";
import { Architecture } from "@/components/marketing/Architecture";
import { QuickStart } from "@/components/marketing/QuickStart";
import { Button } from "@/components/ui/Button";

export function HomePage() {
    return (
        <div className="min-h-screen bg-dark-bg text-text-primary selection:bg-accent/20">
            <Navbar />
            <main>
                <Hero />
                <Features />
                <TerminalDemo />
                <Architecture />
                <QuickStart />

                {/* CTA Section */}
                <section className="py-24 relative overflow-hidden text-center">
                    <div className="absolute inset-0 bg-gradient-to-b from-dark-bg via-accent/5 to-transparent pointer-events-none" />
                    <div className="container mx-auto px-4 relative z-10 space-y-8">
                        <h2 className="text-4xl md:text-6xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-white via-white to-white/50">
                            Stop Guessing. <br /> Start Seeing.
                        </h2>
                        <div className="flex justify-center gap-4">
                            <Button size="lg" className="rounded-full px-12 h-14 text-lg shadow-[0_0_40px_rgba(0,212,255,0.4)] hover:shadow-[0_0_60px_rgba(0,212,255,0.6)] hover:scale-105 transition-all">
                                Deploy Sennet Now
                            </Button>
                        </div>
                    </div>
                </section>
            </main>
            <Footer />
        </div>
    );
}
