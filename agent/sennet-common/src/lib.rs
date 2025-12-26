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

// ============================================================================
// Drop Event Types (Phase 6.1: kfree_skb Tracepoint)
// ============================================================================

/// Drop reason codes (subset of kernel sk_drop_reason, Linux 5.17+)
/// Full list: https://elixir.bootlin.com/linux/latest/source/include/net/dropreason.h
pub mod drop_reason {
    pub const NOT_SPECIFIED: u32 = 1;
    pub const NO_SOCKET: u32 = 2;
    pub const PKT_TOO_SMALL: u32 = 3;
    pub const TCP_CSUM: u32 = 4;
    pub const SOCKET_FILTER: u32 = 5;
    pub const UDP_CSUM: u32 = 6;
    pub const NETFILTER_DROP: u32 = 7;
    pub const OTHERHOST: u32 = 8;
    pub const IP_CSUM: u32 = 9;
    pub const IP_INHDR: u32 = 10;
    pub const IP_RPFILTER: u32 = 11;
    pub const UNICAST_IN_L2_MULTICAST: u32 = 12;
    pub const XFRM_POLICY: u32 = 13;
    pub const IP_NOPROTO: u32 = 14;
    pub const SOCKET_RCVBUFF: u32 = 15;
    pub const PROTO_MEM: u32 = 16;
    pub const TCP_MD5NOTFOUND: u32 = 17;
    pub const TCP_MD5UNEXPECTED: u32 = 18;
    pub const TCP_MD5FAILURE: u32 = 19;
    pub const SOCKET_BACKLOG: u32 = 20;
    pub const TCP_FLAGS: u32 = 21;
    pub const TCP_ZEROWINDOW: u32 = 22;
    pub const TCP_OLD_DATA: u32 = 23;
    pub const TCP_OVERWINDOW: u32 = 24;
    pub const TCP_OFOMERGE: u32 = 25;
    pub const TCP_RFC7323_PAWS: u32 = 26;
    pub const TCP_INVALID_SEQUENCE: u32 = 27;
    pub const TCP_RESET: u32 = 28;
    pub const TCP_INVALID_SYN: u32 = 29;
    pub const TCP_CLOSE: u32 = 30;
    pub const TCP_FASTOPEN: u32 = 31;
    pub const TCP_OLD_ACK: u32 = 32;
    pub const TCP_TOO_OLD_ACK: u32 = 33;
    pub const TCP_ACK_UNSENT_DATA: u32 = 34;
    pub const TCP_OFO_QUEUE_PRUNE: u32 = 35;
    pub const TCP_OFO_DROP: u32 = 36;
    pub const IP_OUTNOROUTES: u32 = 37;
    pub const BPF_CGROUP_EGRESS: u32 = 38;
    pub const IPV6DISABLED: u32 = 39;
    pub const NEIGH_CREATEFAIL: u32 = 40;
    pub const NEIGH_FAILED: u32 = 41;
    pub const NEIGH_QUEUEFULL: u32 = 42;
    pub const NEIGH_DEAD: u32 = 43;
    pub const TC_EGRESS: u32 = 44;
    // Add more as needed from kernel headers
}

/// Event for packet drops (captured from kfree_skb tracepoint)
#[repr(C)]
#[derive(Clone, Copy, Default)]
pub struct DropEvent {
    /// Kernel timestamp in nanoseconds
    pub timestamp_ns: u64,
    /// Drop reason (sk_drop_reason enum value)
    pub reason: u32,
    /// Interface index where drop occurred
    pub ifindex: u32,
    /// Protocol (ETH_P_IP=0x0800, ETH_P_IPV6=0x86DD, etc.)
    pub protocol: u16,
    /// Padding for alignment
    pub _pad: u16,
}

/// Human-readable drop reason string
#[cfg(not(feature = "no-std"))]
pub fn drop_reason_str(reason: u32) -> &'static str {
    use drop_reason::*;
    match reason {
        NOT_SPECIFIED => "NOT_SPECIFIED",
        NO_SOCKET => "NO_SOCKET",
        PKT_TOO_SMALL => "PKT_TOO_SMALL",
        TCP_CSUM => "TCP_CSUM",
        SOCKET_FILTER => "SOCKET_FILTER",
        UDP_CSUM => "UDP_CSUM",
        NETFILTER_DROP => "NETFILTER_DROP",
        OTHERHOST => "OTHERHOST",
        IP_CSUM => "IP_CSUM",
        IP_INHDR => "IP_INHDR",
        IP_RPFILTER => "IP_RPFILTER",
        XFRM_POLICY => "XFRM_POLICY",
        IP_NOPROTO => "IP_NOPROTO",
        SOCKET_RCVBUFF => "SOCKET_RCVBUFF",
        PROTO_MEM => "PROTO_MEM",
        SOCKET_BACKLOG => "SOCKET_BACKLOG",
        TCP_FLAGS => "TCP_FLAGS",
        TCP_ZEROWINDOW => "TCP_ZEROWINDOW",
        TCP_OLD_DATA => "TCP_OLD_DATA",
        TCP_OVERWINDOW => "TCP_OVERWINDOW",
        TCP_RESET => "TCP_RESET",
        TCP_INVALID_SEQUENCE => "TCP_INVALID_SEQ",
        TCP_CLOSE => "TCP_CLOSE",
        IP_OUTNOROUTES => "IP_OUTNOROUTES",
        BPF_CGROUP_EGRESS => "BPF_CGROUP_EGRESS",
        NEIGH_FAILED => "NEIGH_FAILED",
        NEIGH_QUEUEFULL => "NEIGH_QUEUEFULL",
        TC_EGRESS => "TC_EGRESS",
        _ => "UNKNOWN",
    }
}
