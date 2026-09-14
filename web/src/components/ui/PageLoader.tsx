import { SennetMark } from "@/components/brand/SennetLogo";

export function PageLoader({ label = "Resolving product surface", context = "Route / interface / evidence" }: { label?: string; context?: string }) {
    return (
        <main className="signal-loader" role="status" aria-live="polite" aria-label={label}>
            <div className="signal-loader__grid" aria-hidden="true" />
            <header className="signal-loader__brand">
                <SennetMark title="" />
                <strong>SENNET</strong>
                <span>CONNECTED OBSERVABILITY</span>
            </header>
            <div className="signal-loader__stage">
                <div className="signal-loader__index" aria-hidden="true">00 / RESOLVE</div>
                <svg viewBox="0 0 760 260" aria-hidden="true">
                    <path className="signal-loader__rail" d="M28 134h128c82 0 76-88 160-88h111c78 0 73 72 153 72h152c42 0 53 32 100 32" />
                    <path className="signal-loader__rail signal-loader__rail--lower" d="M156 134c75 0 75 82 158 82h110c80 0 78-66 156-66h152" />
                    <path className="signal-loader__route" pathLength="1" d="M28 134h128c82 0 76-88 160-88h111c78 0 73 72 153 72h152c42 0 53 32 100 32" />
                    <g className="signal-loader__nodes">
                        <circle cx="28" cy="134" r="8" />
                        <circle cx="156" cy="134" r="5" />
                        <circle cx="316" cy="46" r="5" />
                        <circle cx="427" cy="46" r="5" />
                        <circle cx="580" cy="118" r="5" />
                        <circle cx="732" cy="150" r="8" />
                    </g>
                </svg>
                <div className="signal-loader__readout">
                    <span>{label}</span>
                    <code>{context}</code>
                </div>
            </div>
            <footer className="signal-loader__footer"><span>FOLLOW THE EVIDENCE</span><i /><span>CONTEXT PRESERVED</span></footer>
        </main>
    );
}
