//! Common types shared between userspace agent and eBPF programs
//!
//! This crate is compiled for both targets.

#![cfg_attr(feature = "no-std", no_std)]

/// Packet statistics counters
#[repr(C)]
#[derive(Clone, Copy, Default)]
pub struct PacketCounters {
    /// Total received packets
    pub rx_packets: u64,
    /// Total received bytes
    pub rx_bytes: u64,
    /// Total transmitted packets  
    pub tx_packets: u64,
    /// Total transmitted bytes
    pub tx_bytes: u64,
    /// Dropped packets
    pub drop_count: u64,
}

/// Event types for RingBuf
#[repr(u32)]
#[derive(Clone, Copy)]
pub enum EventType {
    /// Large packet detected
    LargePacket = 1,
    /// Anomaly detected
    Anomaly = 2,
}

/// Event sent via RingBuf
#[repr(C)]
#[derive(Clone, Copy)]
pub struct PacketEvent {
    /// Event type
    pub event_type: u32,
    /// Packet size in bytes
    pub size: u32,
    /// Source IP (network byte order)
    pub src_ip: u32,
    /// Destination IP (network byte order)
    pub dst_ip: u32,
    /// Protocol (TCP=6, UDP=17, etc)
    pub protocol: u8,
    /// Padding for alignment
    pub _pad: [u8; 3],
}
