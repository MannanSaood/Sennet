//! Sennet eBPF TC Classifier & Drop Tracer
//!
//! This program attaches to:
//! 1. TC (Traffic Control) hook - counts packets/bytes for ingress/egress
//! 2. kfree_skb tracepoint - captures packet drop reasons (Phase 6.1)
//! 3. nf_hook_slow tracepoint - captures netfilter hook/verdict (Phase 6.2)

#![no_std]
#![no_main]

use aya_ebpf::{
    bindings::TC_ACT_PIPE,
    macros::{classifier, map, tracepoint},
    maps::{PerCpuArray, RingBuf},
    programs::{TcContext, TracePointContext},
    helpers::bpf_ktime_get_ns,
};
// use aya_log_ebpf::info; // Reserved for future logging
use sennet_common::{PacketCounters, PacketEvent, DropEvent, NetfilterEvent};

/// Per-CPU counters for packet statistics
/// Index 0 = ingress, Index 1 = egress
#[map]
static COUNTERS: PerCpuArray<PacketCounters> = PerCpuArray::with_max_entries(2, 0);

/// Ring buffer for events (large packets, anomalies)
#[map]
static EVENTS: RingBuf = RingBuf::with_byte_size(256 * 1024, 0); // 256KB

/// Ring buffer for drop events (Phase 6.1)
#[map]
static DROP_EVENTS: RingBuf = RingBuf::with_byte_size(64 * 1024, 0); // 64KB

/// Ring buffer for netfilter events (Phase 6.2)
#[map]
static NF_EVENTS: RingBuf = RingBuf::with_byte_size(32 * 1024, 0); // 32KB

/// Large packet threshold (bytes)
const LARGE_PACKET_THRESHOLD: u32 = 9000; // Jumbo frame size

// =============================================================================
// TC Classifiers (Traffic Counting)
// =============================================================================

/// TC classifier for ingress traffic
#[classifier]
pub fn tc_ingress(ctx: TcContext) -> i32 {
    match process_packet(&ctx, 0) {
        Ok(ret) => ret,
        Err(_) => TC_ACT_PIPE,
    }
}

/// TC classifier for egress traffic
#[classifier]
pub fn tc_egress(ctx: TcContext) -> i32 {
    match process_packet(&ctx, 1) {
        Ok(ret) => ret,
        Err(_) => TC_ACT_PIPE,
    }
}

/// Process a packet and update counters
#[inline(always)]
fn process_packet(ctx: &TcContext, direction: u32) -> Result<i32, ()> {
    let len = ctx.len() as u64;

    // Update counters
    if let Some(counters) = COUNTERS.get_ptr_mut(direction) {
        let counters = unsafe { &mut *counters };
        if direction == 0 {
            // Ingress
            counters.rx_packets += 1;
            counters.rx_bytes += len;
        } else {
            // Egress
            counters.tx_packets += 1;
            counters.tx_bytes += len;
        }
    }

    // Check for large packets and emit event
    if len > LARGE_PACKET_THRESHOLD as u64 {
        emit_large_packet_event(ctx, len as u32)?;
    }

    // TC_ACT_PIPE = pass to next filter/continue
    Ok(TC_ACT_PIPE)
}

/// Emit a large packet event to ring buffer
#[inline(always)]
fn emit_large_packet_event(_ctx: &TcContext, size: u32) -> Result<(), ()> {
    // Try to reserve space in ring buffer
    if let Some(mut entry) = EVENTS.reserve::<PacketEvent>(0) {
        let event = entry.as_mut_ptr();
        unsafe {
            (*event).event_type = 1; // LargePacket
            (*event).size = size;
            
            // Simple IPv4 parsing (assuming Ethernet header is 14 bytes)
            // Offset 14+12=26 (Src IP), 14+16=30 (Dst IP)
            // Note: In real world, need to check EthType and proper bounds
            let src_offset = 14 + 12; // Eth(14) + IP_Offset(12)
            let dst_offset = 14 + 16;
            
            // Default to 0 if we can't read
            (*event).src_ip = _ctx.load(src_offset).unwrap_or(0);
            (*event).dst_ip = _ctx.load(dst_offset).unwrap_or(0);
            (*event).protocol = _ctx.load(14 + 9).unwrap_or(0); // Protocol at offset 9
            
            (*event)._pad = [0; 3];
        }
        entry.submit(0);
    }
    Ok(())
}

// =============================================================================
// kfree_skb Tracepoint (Phase 6.1: Drop Reason Tracing)
// =============================================================================

/// Tracepoint for kernel packet drops
/// 
/// Attaches to: tracepoint/skb/kfree_skb
/// 
/// Context format (Linux 5.17+):
///   struct {
///       void *skbaddr;           // offset 0
///       void *location;          // offset 8
///       unsigned short protocol; // offset 16
///       enum skb_drop_reason reason; // offset 20 (Linux 5.17+)
///   }
#[tracepoint]
pub fn kfree_skb(ctx: TracePointContext) -> u32 {
    match try_kfree_skb(&ctx) {
        Ok(ret) => ret,
        Err(_) => 0,
    }
}

#[inline(always)]
fn try_kfree_skb(ctx: &TracePointContext) -> Result<u32, ()> {
    // Read drop reason from tracepoint context
    // Note: Offset 20 is for Linux 5.17+ where sk_drop_reason is available
    // On older kernels, this field doesn't exist and we'll get garbage/0
    let reason: u32 = unsafe { ctx.read_at(20).unwrap_or(0) };
    
    // Only emit events for interesting drop reasons (not NOT_SPECIFIED=1)
    // Reason 0 means we couldn't read it (older kernel)
    if reason > 1 {
        if let Some(mut entry) = DROP_EVENTS.reserve::<DropEvent>(0) {
            let event = entry.as_mut_ptr();
            unsafe {
                (*event).timestamp_ns = bpf_ktime_get_ns();
                (*event).reason = reason;
                // Protocol is at offset 16 (unsigned short)
                (*event).protocol = ctx.read_at(16).unwrap_or(0);
                (*event).ifindex = 0; // TODO: Extract from skb if needed
                (*event)._pad = 0;
            }
            entry.submit(0);
        }
    }
    
    Ok(0)
}

// =============================================================================
// nf_hook_slow Tracepoint (Phase 6.2: Netfilter Hook Tracing)
// =============================================================================

/// Tracepoint for netfilter slow path processing
/// 
/// Attaches to: tracepoint/netfilter/nf_hook_slow_start or similar
/// 
/// This captures when packets traverse netfilter hooks and their verdicts.
/// Note: The exact tracepoint name and format varies by kernel version.
#[tracepoint]
pub fn nf_hook_slow(ctx: TracePointContext) -> u32 {
    match try_nf_hook_slow(&ctx) {
        Ok(ret) => ret,
        Err(_) => 0,
    }
}

#[inline(always)]
fn try_nf_hook_slow(ctx: &TracePointContext) -> Result<u32, ()> {
    // Read hook state from tracepoint context
    // struct nf_hook_state layout (approximate, varies by kernel):
    //   u8 hook;      // offset 0
    //   u8 pf;        // offset 1
    //   ... other fields
    let hook: u8 = unsafe { ctx.read_at(0).unwrap_or(255) };
    let pf: u8 = unsafe { ctx.read_at(1).unwrap_or(0) };
    
    // For nf_hook_slow_finish tracepoint, verdict is typically at a later offset
    // For now, we'll record all hook invocations
    let verdict: u8 = unsafe { ctx.read_at(8).unwrap_or(1) }; // Default ACCEPT=1
    
    // Only record DROP events or interesting hooks
    if verdict == 0 || hook <= 4 { // NF_DROP or valid hook types
        if let Some(mut entry) = NF_EVENTS.reserve::<NetfilterEvent>(0) {
            let event = entry.as_mut_ptr();
            unsafe {
                (*event).timestamp_ns = bpf_ktime_get_ns();
                (*event).hook = hook;
                (*event).pf = pf;
                (*event).verdict = verdict;
                (*event)._pad = 0;
                (*event).ifindex_in = 0;  // TODO: Extract from context
                (*event).ifindex_out = 0; // TODO: Extract from context
            }
            entry.submit(0);
        }
    }
    
    Ok(0)
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    unsafe { core::hint::unreachable_unchecked() }
}
