import { useEffect, useRef } from "react";

type Node = {
    x: number;
    y: number;
    baseX: number;
    baseY: number;
    vx: number;
    vy: number;
    radius: number;
    phase: number; // For breathing effect
    connections: number[];
};

type Packet = {
    fromIndex: number;
    toIndex: number;
    progress: number;
    speed: number;
};

export function NetworkAnimation() {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    const mouseRef = useRef({ x: 0, y: 0 });

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;

        const ctx = canvas.getContext("2d");
        if (!ctx) return;

        let nodes: Node[] = [];
        let packets: Packet[] = [];
        let animationFrameId: number;
        let time = 0;

        const NODE_COUNT = 45;
        const CONNECTION_DISTANCE = 180;
        const MOUSE_RADIUS = 250;

        // Gradient definitions
        const accentColor = "0, 212, 255"; // #00D4FF

        const resize = () => {
            if (containerRef.current) {
                canvas.width = containerRef.current.clientWidth;
                canvas.height = containerRef.current.clientHeight;
                initNodes();
            }
        };

        const initNodes = () => {
            nodes = [];
            packets = [];
            for (let i = 0; i < NODE_COUNT; i++) {
                const x = Math.random() * canvas.width;
                const y = Math.random() * canvas.height;
                nodes.push({
                    x, y,
                    baseX: x,
                    baseY: y,
                    vx: (Math.random() - 0.5) * 0.2,
                    vy: (Math.random() - 0.5) * 0.2,
                    radius: Math.random() * 1.5 + 1,
                    phase: Math.random() * Math.PI * 2,
                    connections: [],
                });
            }
        };

        const handleMouseMove = (e: MouseEvent) => {
            if (!canvasRef.current) return;
            const rect = canvasRef.current.getBoundingClientRect();
            mouseRef.current = {
                x: e.clientX - rect.left,
                y: e.clientY - rect.top
            };
        };

        const draw = () => {
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            time += 0.01;

            // Update nodes
            const connections: [number, number, number][] = [];

            nodes.forEach((node, i) => {
                // Floating motion
                node.x = node.baseX + Math.sin(time + node.phase) * 20;
                node.y = node.baseY + Math.cos(time + node.phase * 0.5) * 20;

                // Mouse interaction
                const dx = mouseRef.current.x - node.x;
                const dy = mouseRef.current.y - node.y;
                const dist = Math.sqrt(dx * dx + dy * dy);

                if (dist < MOUSE_RADIUS) {
                    const force = (MOUSE_RADIUS - dist) / MOUSE_RADIUS;
                    const angle = Math.atan2(dy, dx);
                    // Repel slightly
                    node.x -= Math.cos(angle) * force * 30;
                    node.y -= Math.sin(angle) * force * 30;
                }

                // Connections
                for (let j = i + 1; j < nodes.length; j++) {
                    const other = nodes[j];
                    const dx = node.x - other.x;
                    const dy = node.y - other.y;
                    const dist = Math.sqrt(dx * dx + dy * dy);

                    if (dist < CONNECTION_DISTANCE) {
                        connections.push([i, j, dist]);

                        // Randomly spawn packet
                        if (packets.length < 30 && Math.random() < 0.002) {
                            packets.push({
                                fromIndex: i,
                                toIndex: j,
                                progress: 0,
                                speed: 0.01 + Math.random() * 0.01,
                            });
                        }
                    }
                }
            });

            // Draw connections
            ctx.globalCompositeOperation = "screen"; // Additive blending for glow
            connections.forEach(([i, j, dist]) => {
                const opacity = 1 - dist / CONNECTION_DISTANCE;

                ctx.strokeStyle = `rgba(148, 163, 184, ${opacity * 0.15})`;
                ctx.beginPath();
                ctx.moveTo(nodes[i].x, nodes[i].y);
                ctx.lineTo(nodes[j].x, nodes[j].y);
                ctx.stroke();
            });

            // Draw packets (glowing pulses)
            packets = packets.filter(p => p.progress < 1);
            packets.forEach(p => {
                p.progress += p.speed;
                const start = nodes[p.fromIndex];
                const end = nodes[p.toIndex];

                const x = start.x + (end.x - start.x) * p.progress;
                const y = start.y + (end.y - start.y) * p.progress;

                // Trail effect
                const gradient = ctx.createLinearGradient(
                    x - (end.x - start.x) * 0.1,
                    y - (end.y - start.y) * 0.1,
                    x, y
                );
                gradient.addColorStop(0, "rgba(0, 212, 255, 0)");
                gradient.addColorStop(1, "rgba(0, 212, 255, 0.8)");

                ctx.strokeStyle = gradient;
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(x - (end.x - start.x) * 0.05, y - (end.y - start.y) * 0.05);
                ctx.lineTo(x, y);
                ctx.stroke();

                // Head
                ctx.fillStyle = `rgba(${accentColor}, 1)`;
                ctx.shadowBlur = 8;
                ctx.shadowColor = `rgba(${accentColor}, 0.8)`;
                ctx.beginPath();
                ctx.arc(x, y, 1.5, 0, Math.PI * 2);
                ctx.fill();
                ctx.shadowBlur = 0;
            });

            // Draw nodes
            nodes.forEach((node) => {
                ctx.fillStyle = `rgba(255, 255, 255, ${0.3 + Math.sin(time + node.phase) * 0.2})`;
                ctx.beginPath();
                ctx.arc(node.x, node.y, node.radius, 0, Math.PI * 2);
                ctx.fill();
            });
            ctx.globalCompositeOperation = "source-over";

            animationFrameId = requestAnimationFrame(draw);
        };

        // Initialize
        resize();
        window.addEventListener("resize", resize);
        window.addEventListener("mousemove", handleMouseMove);
        draw();

        return () => {
            window.removeEventListener("resize", resize);
            window.removeEventListener("mousemove", handleMouseMove);
            cancelAnimationFrame(animationFrameId);
        };
    }, []);

    return (
        <div ref={containerRef} className="absolute inset-0 z-0 pointer-events-none fade-in duration-1000">
            <canvas ref={canvasRef} />
        </div>
    );
}
