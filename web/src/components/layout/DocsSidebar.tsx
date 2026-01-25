import { Link, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";

const sidebarNav = [
    {
        title: "Getting Started",
        items: [
            { title: "Introduction", href: "/docs" },
            { title: "Quick Start", href: "/docs/quickstart" },
            { title: "Installation", href: "/docs/installation" },
            { title: "Configuration", href: "/docs/configuration" },
        ],
    },
    {
        title: "Core Concepts",
        items: [
            { title: "Architecture", href: "/docs/architecture" },
            { title: "eBPF Basics", href: "/docs/ebpf" },
            { title: "Metrics & Data", href: "/docs/metrics" },
        ],
    },
    {
        title: "Reference",
        items: [
            { title: "CLI Commands", href: "/docs/cli" },
            { title: "API Reference", href: "/docs/api" },
        ],
    },
];

export function DocsSidebar() {
    const location = useLocation();

    return (
        <div className="fixed top-16 bottom-0 left-0 w-64 border-r border-dark-border bg-dark-bg hidden md:block overflow-y-auto py-8 px-6">
            <div className="space-y-8">
                {sidebarNav.map((section, i) => (
                    <div key={i}>
                        <h4 className="mb-3 text-sm font-semibold text-text-primary tracking-wide">
                            {section.title}
                        </h4>
                        <ul className="space-y-2">
                            {section.items.map((item) => (
                                <li key={item.href}>
                                    <Link
                                        to={item.href}
                                        className={cn(
                                            "block text-sm transition-colors hover:text-accent",
                                            location.pathname === item.href
                                                ? "text-accent font-medium"
                                                : "text-text-secondary"
                                        )}
                                    >
                                        {item.title}
                                    </Link>
                                </li>
                            ))}
                        </ul>
                    </div>
                ))}
            </div>
        </div>
    );
}
