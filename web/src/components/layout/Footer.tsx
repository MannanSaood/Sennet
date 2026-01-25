import { Link } from "react-router-dom";
import { Shield, Github, Twitter, Linkedin } from "lucide-react";

export function Footer() {
    return (
        <footer className="bg-dark-surface border-t border-dark-border">
            <div className="container mx-auto px-4 py-12 md:py-16">
                <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-8 mb-12">
                    <div className="col-span-2 lg:col-span-2">
                        <Link to="/" className="flex items-center gap-2 mb-4">
                            <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-gradient-to-br from-accent to-accent-alt text-white">
                                <Shield className="w-5 h-5" />
                            </div>
                            <span className="text-xl font-bold text-text-primary">
                                Sennet
                            </span>
                        </Link>
                        <p className="text-text-secondary text-sm max-w-xs mb-6">
                            Next-gen network observability platform powered by eBPF.
                            See every packet, trace every drop, understand your infrastructure.
                        </p>
                        <div className="flex items-center gap-4">
                            <a href="https://github.com" target="_blank" rel="noopener noreferrer" className="text-text-secondary hover:text-text-primary transition-colors">
                                <Github className="w-5 h-5" />
                            </a>
                            <a href="https://twitter.com" target="_blank" rel="noopener noreferrer" className="text-text-secondary hover:text-text-primary transition-colors">
                                <Twitter className="w-5 h-5" />
                            </a>
                            <a href="https://linkedin.com" target="_blank" rel="noopener noreferrer" className="text-text-secondary hover:text-text-primary transition-colors">
                                <Linkedin className="w-5 h-5" />
                            </a>
                        </div>
                    </div>

                    <div>
                        <h4 className="font-semibold text-text-primary mb-4">Product</h4>
                        <ul className="space-y-2 text-sm">
                            <li><Link to="/#features" className="text-text-secondary hover:text-accent transition-colors">Features</Link></li>
                            <li><Link to="/#architecture" className="text-text-secondary hover:text-accent transition-colors">Architecture</Link></li>
                            <li><Link to="/dashboard" className="text-text-secondary hover:text-accent transition-colors">Dashboard</Link></li>
                            <li><Link to="/changelog" className="text-text-secondary hover:text-accent transition-colors">Changelog</Link></li>
                        </ul>
                    </div>

                    <div>
                        <h4 className="font-semibold text-text-primary mb-4">Resources</h4>
                        <ul className="space-y-2 text-sm">
                            <li><Link to="/docs" className="text-text-secondary hover:text-accent transition-colors">Documentation</Link></li>
                            <li><Link to="/docs/quickstart" className="text-text-secondary hover:text-accent transition-colors">Quick Start</Link></li>
                            <li><Link to="/blog" className="text-text-secondary hover:text-accent transition-colors">Blog</Link></li>
                            <li><Link to="/community" className="text-text-secondary hover:text-accent transition-colors">Community</Link></li>
                        </ul>
                    </div>

                    <div>
                        <h4 className="font-semibold text-text-primary mb-4">Company</h4>
                        <ul className="space-y-2 text-sm">
                            <li><Link to="/about" className="text-text-secondary hover:text-accent transition-colors">About</Link></li>
                            <li><Link to="/careers" className="text-text-secondary hover:text-accent transition-colors">Careers</Link></li>
                            <li><Link to="/legal/privacy" className="text-text-secondary hover:text-accent transition-colors">Privacy</Link></li>
                            <li><Link to="/legal/terms" className="text-text-secondary hover:text-accent transition-colors">Terms</Link></li>
                        </ul>
                    </div>
                </div>

                <div className="pt-8 border-t border-dark-border flex flex-col md:flex-row items-center justify-between text-sm text-text-secondary">
                    <p>&copy; {new Date().getFullYear()} Sennet Inc. All rights reserved.</p>
                    <div className="flex items-center gap-6 mt-4 md:mt-0">
                        <Link to="/privacy" className="hover:text-text-primary">Privacy Policy</Link>
                        <Link to="/terms" className="hover:text-text-primary">Terms of Service</Link>
                    </div>
                </div>
            </div>
        </footer>
    );
}
