# Two-minute overview

The production-video slot above is intentionally waiting for footage recorded from the final product. Its source paths, poster, captions, chapters, transcript, duration, fullscreen control, responsive behavior, and fallback text are already wired.

## What the walkthrough will cover

1. Start with a failed checkout request.
2. Expand its distributed trace and explicit span links.
3. Inspect a parallel agent handoff and retry.
4. Correlate the network delay and settlement correction.
5. Save the investigation as a versioned dashboard.

> **Evidence rule:** final footage must be recorded from the working local product. Do not replace the missing files with concept animation or simulated customer telemetry.

## Video delivery contract

The video-production workstream can place final files at:

```text
web/public/media/docs/sennet-two-minute-overview.mp4
web/public/media/docs/sennet-two-minute-overview.webm
web/public/media/docs/sennet-two-minute-overview.en.vtt
```

After review, set the `ready` field for this page in `src/content/docs/navigation.ts` to `true`.
