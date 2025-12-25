//! Sennet eBPF TC Classifier
//!
//! This program attaches to the TC (Traffic Control) hook and counts
//! packets/bytes for both ingress and egress traffic.

#![no_std]
#![no_main]

use aya_ebpf::{
    bindings::TC_ACT_PIPE,
    macros::{classifier, map},
    maps::{PerCpuArray, RingBuf},
    programs::TcContext,
};
// use aya_log_ebpf::info; // Reserved for future logging
use sennet_common::{PacketCounters, PacketEvent};

/// Per-CPU counters for packet statistics
/// Index 0 = ingress, Index 1 = egress
#[map]
static COUNTERS: PerCpuArray<PacketCounters> = PerCpuArray::with_max_entries(2, 0);

/// Ring buffer for events (large packets, anomalies)
#[map]
static EVENTS: RingBuf = RingBuf::with_byte_size(256 * 1024, 0); // 256KB

/// Large packet threshold (bytes)
const LARGE_PACKET_THRESHOLD: u32 = 9000; // Jumbo frame size

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
            // TODO: Parse IP header to get src/dst
            (*event).src_ip = 0;
            (*event).dst_ip = 0;
            (*event).protocol = 0;
            (*event)._pad = [0; 3];
        }
        entry.submit(0);
    }
    Ok(())
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    unsafe { core::hint::unreachable_unchecked() }
}
