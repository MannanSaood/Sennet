import { Navbar } from "@/components/layout/Navbar";
import { DocsSidebar } from "@/components/layout/DocsSidebar";
import { Footer } from "@/components/layout/Footer";

export function DocsLayout({ children }: { children: React.ReactNode }) {
    return (
        <div className="min-h-screen bg-dark-bg text-text-primary flex flex-col">
            <Navbar />
            <div className="flex-1 flex pt-16">
                <DocsSidebar />
                <main className="flex-1 md:pl-64 min-w-0">
                    <div className="container mx-auto px-4 py-8 md:py-12 max-w-4xl lg:px-12">
                        {children}
                    </div>
                </main>
            </div>
            <div className="md:pl-64">
                <Footer />
            </div>
        </div>
    );
}
