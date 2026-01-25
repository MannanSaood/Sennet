# Introduction to Sennet

Sennet is a next-generation network observability platform powered by eBPF. It provides granular visibility into your infrastructure without the overhead of sidecars or heavy agents.

## Why Sennet?

- **Zero Overhead**: Runs in kernel space using eBPF.
- **Deep Visibility**: See every packet, flow, and drop.
- **K8s Native**: Understands Pods, Services, and Namespaces.
- **Developer Friendly**: Beautiful CLI and modern Dashboard.

## How it Works

Sennet loads small, safe programs into the Linux kernel that hook into the networking stack (Traffic Control layer). These programs aggregate metrics and send them to userspace asynchronously, ensuring your application performance is never impacted.

```rust
// Simplified view of what happens in the kernel
fn handle_ingress(skb: SkBuff) -> Result<Action, Error> {
    let packet = skb.parse()?;
    metrics.increment(packet.protocol, packet.len);
    Ok(Action::Continue)
}
```
