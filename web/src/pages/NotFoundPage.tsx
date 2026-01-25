import { Link } from "react-router-dom";
import { Button } from "@/components/ui/Button";
import { Ghost, ArrowLeft } from "lucide-react";

export function NotFoundPage() {
    return (
        <div className="min-h-screen flex flex-col items-center justify-center bg-dark-bg text-white p-4 text-center relative overflow-hidden">
            {/* Background decoration */}
            <div className="absolute inset-0 pointer-events-none opacity-20"
                style={{ backgroundImage: 'radial-gradient(circle at center, #00D4FF 0%, transparent 70%)', transform: 'scale(1.5)' }}
            />

            <div className="relative z-10 flex flex-col items-center">
                <div className="mb-8 relative">
                    <Ghost className="w-32 h-32 text-text-secondary opacity-50 animate-bounce" />
                    <div className="absolute -bottom-4 left-1/2 -translate-x-1/2 w-16 h-4 bg-black/50 blur-lg rounded-full" />
                </div>

                <h1 className="text-7xl font-bold bg-clip-text text-transparent bg-gradient-to-b from-white to-white/50 mb-4">
                    404
                </h1>

                <h2 className="text-2xl font-semibold mb-2">Page Not Found</h2>
                <p className="text-text-secondary max-w-md mb-8">
                    The page you are looking for might have been removed, had its name changed, or is temporarily unavailable.
                </p>

                <div className="flex gap-4">
                    <Link to="/">
                        <Button className="gap-2">
                            <ArrowLeft className="w-4 h-4" />
                            Back to Home
                        </Button>
                    </Link>
                </div>
            </div>
        </div>
    );
}
