import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { Button } from "@/components/ui/Button";
import { AlertTriangle, RefreshCw } from "lucide-react";

interface Props {
    children: ReactNode;
}

interface State {
    hasError: boolean;
    error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
    public state: State = {
        hasError: false,
        error: null,
    };

    public static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error };
    }

    public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        console.error("Uncaught error:", error, errorInfo);
    }

    public render() {
        if (this.state.hasError) {
            return (
                <div className="min-h-screen flex flex-col items-center justify-center bg-dark-bg text-white p-4 text-center">
                    <div className="bg-error/10 p-4 rounded-full mb-6">
                        <AlertTriangle className="w-12 h-12 text-error" />
                    </div>
                    <h1 className="text-3xl font-bold mb-2">Something went wrong</h1>
                    <p className="text-text-secondary max-w-md mb-8">
                        An unexpected error occurred. Our team has been notified.
                        <br />
                        <span className="text-xs font-mono mt-2 block bg-black/30 p-2 rounded text-error/80">
                            {this.state.error?.message}
                        </span>
                    </p>
                    <div className="flex gap-4">
                        <Button onClick={() => window.location.reload()} className="gap-2">
                            <RefreshCw className="w-4 h-4" />
                            Reload Application
                        </Button>
                        <Button variant="secondary" onClick={() => window.location.href = '/'}>
                            Go Home
                        </Button>
                    </div>
                </div>
            );
        }

        return this.props.children;
    }
}
