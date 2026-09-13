//! Portable Sennet eBPF data plane.
//!
//! Only TC metadata supplied by the verifier (`len`) is consumed. Kernel
//! structure and tracepoint layouts are deliberately not decoded here: doing
//! so without generated CO-RE relocations can silently fabricate telemetry.

#![no_std]
#![no_main]

use aya_ebpf::{
    bindings::TC_ACT_PIPE,
    macros::{classifier, map},
    maps::PerCpuArray,
    programs::TcContext,
};
use sennet_common::PacketCounters;

/// Index 0 is ingress and index 1 is egress.
#[map]
static COUNTERS: PerCpuArray<PacketCounters> = PerCpuArray::with_max_entries(2, 0);

#[classifier]
pub fn tc_ingress(ctx: TcContext) -> i32 {
    count(&ctx, 0)
}

#[classifier]
pub fn tc_egress(ctx: TcContext) -> i32 {
    count(&ctx, 1)
}

#[inline(always)]
fn count(ctx: &TcContext, direction: u32) -> i32 {
    if let Some(counters) = COUNTERS.get_ptr_mut(direction) {
        let counters = unsafe { &mut *counters };
        if direction == 0 {
            counters.rx_packets = counters.rx_packets.saturating_add(1);
            counters.rx_bytes = counters.rx_bytes.saturating_add(ctx.len() as u64);
        } else {
            counters.tx_packets = counters.tx_packets.saturating_add(1);
            counters.tx_bytes = counters.tx_bytes.saturating_add(ctx.len() as u64);
        }
    }
    TC_ACT_PIPE
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    unsafe { core::hint::unreachable_unchecked() }
}
