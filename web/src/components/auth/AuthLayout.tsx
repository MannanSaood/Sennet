import { Link } from "react-router-dom";
import { Shield } from "lucide-react";
import { NetworkAnimation } from "@/components/marketing/NetworkAnimation"; // Reuse the animation

interface AuthLayoutProps {
    children: React.ReactNode;
    title: string;
    description: string;
}

export function AuthLayout({ children, title, description }: AuthLayoutProps) {
    return (
        <div className="container relative h-screen flex-col items-center justify-center grid lg:max-w-none lg:grid-cols-2 lg:px-0">
            {/* Sidebar (Desktop only) */}
            <div className="relative hidden h-full flex-col bg-dark-bg p-10 text-white lg:flex border-r border-dark-border overflow-hidden">
                {/* Visual Background */}
                <div className="absolute inset-0 z-0">
                    <NetworkAnimation />
                    <div className="absolute inset-0 bg-gradient-to-t from-dark-bg to-transparent" />
                </div>

                <div className="relative z-20 flex items-center gap-2 text-lg font-medium">
                    <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-gradient-to-br from-accent to-accent-alt text-white shadow-lg shadow-accent/20">
                        <Shield className="w-5 h-5" />
                    </div>
                    Sennet
                </div>

                {/* Center Content */}
                <div className="relative z-20 m-auto flex flex-col items-center text-center space-y-6 max-w-lg">
                    <h2 className="text-4xl font-bold tracking-tight bg-clip-text text-transparent bg-gradient-to-b from-white to-white/60">
                        Observability Reimagined
                    </h2>
                    <p className="text-lg text-text-secondary leading-relaxed">
                        Join thousands of engineers who trust Sennet for zero-overhead eBPF monitoring.
                    </p>
                </div>

            </div>

            {/* Main Content */}
            <div className="lg:p-8 relative min-h-screen flex flex-col justify-center bg-dark-bg">
                <div className="mx-auto flex w-full flex-col justify-center space-y-6 sm:w-[350px]">
                    <div className="flex flex-col space-y-2 text-center">
                        <h1 className="text-2xl font-semibold tracking-tight text-white">
                            {title}
                        </h1>
                        <p className="text-sm text-text-secondary">
                            {description}
                        </p>
                    </div>

                    {children}

                    <p className="px-8 text-center text-sm text-text-secondary">
                        By clicking continue, you agree to our{" "}
                        <Link to="/terms" className="underline underline-offset-4 hover:text-accent">
                            Terms of Service
                        </Link>{" "}
                        and{" "}
                        <Link to="/privacy" className="underline underline-offset-4 hover:text-accent">
                            Privacy Policy
                        </Link>
                        .
                    </p>
                </div>
            </div>

            {/* Mobile Nav Helper */}
            <div className="absolute top-4 right-4 md:top-8 md:right-8 z-50">
                <Link to="/" className="text-sm font-medium text-text-secondary hover:text-accent transition-colors">
                    Back to Home
                </Link>
            </div>
        </div>
    );
}
