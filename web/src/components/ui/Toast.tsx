import { useToast } from "@/hooks/useToast";
import { X, CheckCircle, AlertTriangle, Info, AlertCircle } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { cn } from "@/lib/utils";

export function Toaster() {
    const { toasts, dismiss } = useToast();

    return (
        <div className="fixed bottom-0 right-0 z-[100] flex flex-col gap-2 p-4 w-full max-w-sm sm:bottom-4 sm:right-4">
            <AnimatePresence mode="popLayout">
                {toasts.map(function ({ id, title, description, action, variant = "default" }) {

                    let Icon = Info;
                    if (variant === "success") Icon = CheckCircle;
                    if (variant === "warning") Icon = AlertTriangle;
                    if (variant === "destructive") Icon = AlertCircle;
                    if (variant === "default") Icon = Info;

                    return (
                        <motion.div
                            key={id}
                            layout
                            initial={{ opacity: 0, x: 20, scale: 0.95 }}
                            animate={{ opacity: 1, x: 0, scale: 1 }}
                            exit={{ opacity: 0, scale: 0.95, transition: { duration: 0.2 } }}
                            className={cn(
                                "group pointer-events-auto relative flex w-full items-start gap-4 overflow-hidden rounded-md border p-4 pr-8 shadow-lg transition-all",
                                variant === "default" && "bg-dark-surface border-dark-border text-text-primary",
                                variant === "destructive" && "bg-error/10 border-error/20 text-error",
                                variant === "success" && "bg-success/10 border-success/20 text-success",
                                variant === "warning" && "bg-warning/10 border-warning/20 text-warning",
                                "backdrop-blur-md"
                            )}
                        >
                            <div className="mt-0.5 shrink-0">
                                <Icon className="h-5 w-5" />
                            </div>

                            <div className="grid gap-1">
                                {title && (
                                    <div className={cn("text-sm font-semibold", variant !== "default" && "text-inherit")}>{title}</div>
                                )}
                                {description && (
                                    <div className={cn("text-sm opacity-90", variant !== "default" && "text-inherit")}>
                                        {description}
                                    </div>
                                )}
                            </div>

                            {action && (
                                <div className="ml-auto flex items-center gap-2">
                                    {action}
                                </div>
                            )}

                            <button
                                onClick={() => dismiss(id)}
                                className={cn(
                                    "absolute right-2 top-2 rounded-md p-1 bg-transparent hover:bg-black/5 transition-colors",
                                    variant === "default" ? "text-text-secondary hover:text-text-primary" : "text-inherit opacity-70 hover:opacity-100"
                                )}
                            >
                                <X className="h-4 w-4" />
                            </button>
                        </motion.div>
                    );
                })}
            </AnimatePresence>
        </div>
    );
}
