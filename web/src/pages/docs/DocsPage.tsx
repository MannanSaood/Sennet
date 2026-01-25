import { useEffect, useState } from "react";
import { useParams, useLocation } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import rehypeRaw from "rehype-raw";
import { DocsLayout } from "./DocsLayout";
import { Loader2 } from "lucide-react";
import "highlight.js/styles/github-dark.css"; // or atom-one-dark

// Glob import all markdown files
const modules = import.meta.glob("/src/content/docs/*.md", { query: "?raw", import: "default" });

export function DocsPage() {
    const { "*": splat } = useParams();
    const [content, setContent] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const location = useLocation();

    useEffect(() => {
        const loadContent = async () => {
            setIsLoading(true);
            try {
                // Default to introduction if root docs path
                const slug = splat || "introduction";
                const path = `/src/content/docs/${slug}.md`;

                const loader = modules[path];

                if (loader) {
                    const text = await loader();
                    setContent(text as string);
                } else {
                    setContent("# 404 Not Found\n\nThis page does not exist.");
                }
            } catch (err) {
                console.error("Failed to load docs:", err);
                setContent("# Error\n\nFailed to load documentation.");
            } finally {
                setIsLoading(false);
            }
        };

        loadContent();
    }, [splat, location.pathname]);

    return (
        <DocsLayout>
            {isLoading ? (
                <div className="flex items-center justify-center h-64">
                    <Loader2 className="w-8 h-8 animate-spin text-accent" />
                </div>
            ) : (
                <article className="prose prose-invert prose-slate max-w-none 
          prose-headings:font-bold prose-headings:tracking-tight 
          prose-h1:text-4xl prose-h1:text-white prose-h1:mb-8
          prose-h2:text-2xl prose-h2:text-white prose-h2:mt-12 prose-h2:mb-4 prose-h2:border-b prose-h2:border-white/10 prose-h2:pb-2
          prose-p:text-text-secondary prose-p:leading-7
          prose-a:text-accent prose-a:no-underline hover:prose-a:underline
          prose-strong:text-white
          prose-code:text-accent-alt prose-code:bg-white/5 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded prose-code:before:content-none prose-code:after:content-none
          prose-pre:bg-[#0d1117] prose-pre:border prose-pre:border-white/10 prose-pre:rounded-xl
        ">
                    <ReactMarkdown
                        remarkPlugins={[remarkGfm]}
                        rehypePlugins={[rehypeRaw, rehypeHighlight]}
                    >
                        {content || ""}
                    </ReactMarkdown>
                </article>
            )}
        </DocsLayout>
    );
}
