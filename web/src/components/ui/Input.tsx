import * as React from "react";
import { cn } from "@/lib/utils";

export interface InputProps
    extends React.InputHTMLAttributes<HTMLInputElement> {
    error?: string;
    label?: string;
    icon?: React.ReactNode;
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
    ({ className, type, error, label, icon, id, ...props }, ref) => {
        const inputId = id || React.useId();

        return (
            <div className="w-full space-y-2">
                {label && (
                    <label
                        htmlFor={inputId}
                        className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 text-text-secondary"
                    >
                        {label}
                    </label>
                )}
                <div className="relative">
                    {icon && (
                        <div className="absolute left-3 top-2.5 text-text-secondary">
                            {icon}
                        </div>
                    )}
                    <input
                        id={inputId}
                        type={type}
                        className={cn(
                            "flex h-10 w-full rounded-md border border-dark-border bg-dark-surface px-3 py-2 text-sm ring-offset-dark-bg file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 text-text-primary",
                            icon && "pl-10",
                            error && "border-error focus-visible:ring-error",
                            className
                        )}
                        ref={ref}
                        {...props}
                    />
                </div>
                {error && (
                    <p className="text-sm font-medium text-error">{error}</p>
                )}
            </div>
        );
    }
);
Input.displayName = "Input";

export { Input };
