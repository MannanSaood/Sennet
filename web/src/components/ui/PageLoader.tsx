import { Loader2 } from "lucide-react";

export function PageLoader() {
    return (
        <div className="min-h-screen flex items-center justify-center bg-dark-bg">
            <div className="flex flex-col items-center gap-4">
                <Loader2 className="w-10 h-10 animate-spin text-accent" />
                <p className="text-sm text-text-secondary animate-pulse">Loading Sennet...</p>
            </div>
        </div>
    );
}
