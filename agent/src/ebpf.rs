//! eBPF Loader and Manager
//!
//! Loads and manages the TC eBPF programs and kfree_skb tracepoint.
//! Reads counters and drop events.
//! On non-Linux platforms, provides a mock implementation.

use anyhow::Result;

/// Packet counters structure (mirrors eBPF side)
/// Must implement Pod trait for use with aya maps
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
pub struct PacketCounters {
    pub rx_packets: u64,
    pub rx_bytes: u64,
    pub tx_packets: u64,
    pub tx_bytes: u64,
    pub drop_count: u64,
}

// SAFETY: PacketCounters is #[repr(C)], contains only u64 fields,
// and has no padding. It is safe to interpret as bytes.
#[cfg(target_os = "linux")]
unsafe impl aya::Pod for PacketCounters {}

/// Drop event structure (mirrors eBPF side)
/// Used for kfree_skb tracepoint events
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
pub struct DropEvent {
    pub timestamp_ns: u64,
    pub reason: u32,
    pub ifindex: u32,
    pub protocol: u16,
    pub _pad: u16,
}

#[cfg(target_os = "linux")]
unsafe impl aya::Pod for DropEvent {}

/// Human-readable drop reason string (from sk_drop_reason enum)
pub fn drop_reason_str(reason: u32) -> &'static str {
    match reason {
        1 => "NOT_SPECIFIED",
        2 => "NO_SOCKET",
        3 => "PKT_TOO_SMALL",
        4 => "TCP_CSUM",
        5 => "SOCKET_FILTER",
        6 => "UDP_CSUM",
        7 => "NETFILTER_DROP",
        8 => "OTHERHOST",
        9 => "IP_CSUM",
        10 => "IP_INHDR",
        11 => "IP_RPFILTER",
        13 => "XFRM_POLICY",
        14 => "IP_NOPROTO",
        15 => "SOCKET_RCVBUFF",
        16 => "PROTO_MEM",
        20 => "SOCKET_BACKLOG",
        21 => "TCP_FLAGS",
        22 => "TCP_ZEROWINDOW",
        23 => "TCP_OLD_DATA",
        24 => "TCP_OVERWINDOW",
        27 => "TCP_INVALID_SEQ",
        28 => "TCP_RESET",
        30 => "TCP_CLOSE",
        37 => "IP_OUTNOROUTES",
        38 => "BPF_CGROUP_EGRESS",
        41 => "NEIGH_FAILED",
        42 => "NEIGH_QUEUEFULL",
        44 => "TC_EGRESS",
        _ => "UNKNOWN",
    }
}

/// Netfilter event structure (mirrors eBPF side)
/// Used for nf_hook_slow tracepoint events (Phase 6.2)
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
pub struct NetfilterEvent {
    pub timestamp_ns: u64,
    pub hook: u8,
    pub pf: u8,
    pub verdict: u8,
    pub _pad: u8,
    pub ifindex_in: u32,
    pub ifindex_out: u32,
}

#[cfg(target_os = "linux")]
unsafe impl aya::Pod for NetfilterEvent {}

#[cfg(target_os = "linux")]
use {
    aya::{
        programs::{tc, SchedClassifier, TcAttachType, TracePoint},
        maps::PerCpuArray,
        Bpf,
    },
    std::path::Path,
};

/// eBPF program manager
pub struct EbpfManager {
    interface: String,
    #[cfg(target_os = "linux")]
    #[allow(dead_code)]
    bpf: Bpf,
    /// Whether drop tracing is active (kfree_skb tracepoint attached)
    pub drop_tracing_enabled: bool,
    /// Whether netfilter tracing is active (nf_hook_slow tracepoint attached)
    pub nf_tracing_enabled: bool,
}

impl EbpfManager {
    /// Load and attach eBPF programs to the specified interface
    #[cfg(target_os = "linux")]
    pub fn load_and_attach(interface: &str) -> Result<Self> {
        tracing::info!("Loading eBPF programs...");
        
        // Load the eBPF binary
        // During CI/Cross build, build.rs copies binary to OUT_DIR/sennet_ebpf.bin
        #[cfg(feature = "embed_bpf")]
        let ebpf_bytes: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/sennet_ebpf.bin"));
        
        #[cfg(not(feature = "embed_bpf"))]
        let ebpf_bytes: &[u8] = include_bytes!("../../sennet-ebpf/target/bpfel-unknown-none/release/sennet-ebpf");
        
        // Debug: Log embedded binary info
        tracing::info!("eBPF binary size: {} bytes", ebpf_bytes.len());
        if ebpf_bytes.len() >= 4 {
            tracing::info!(
                "eBPF ELF magic: {:02x} {:02x} {:02x} {:02x} (expected: 7f 45 4c 46 = ELF)",
                ebpf_bytes[0], ebpf_bytes[1], ebpf_bytes[2], ebpf_bytes[3]
            );
        }
        if ebpf_bytes.len() >= 18 {
            // e_type at offset 16 (2 bytes, little-endian): 3 = shared, 0xff = BPF
            let e_type = u16::from_le_bytes([ebpf_bytes[16], ebpf_bytes[17]]);
            tracing::info!("eBPF ELF e_type: {} (3=ET_DYN/shared, 0xff00=BPF)", e_type);
        }
        
        // Check for BTF sections
        let has_btf = ebpf_bytes.windows(4).any(|w| w == b".BTF");
        tracing::info!("eBPF contains BTF sections: {}", has_btf);
        
        let mut bpf = match Bpf::load(ebpf_bytes) {
            Ok(b) => b,
            Err(e) => {
                // Log detailed error chain
                tracing::error!("eBPF load failed: {}", e);
                tracing::error!("Error debug: {:?}", e);
                // Try to get more details about the error source
                if let Some(source) = std::error::Error::source(&e) {
                    tracing::error!("Caused by: {}", source);
                    if let Some(source2) = std::error::Error::source(source) {
                        tracing::error!("Caused by: {}", source2);
                    }
                }
                return Err(e.into());
            }
        };
        
        // Pin path for maps
        let pin_path = Path::new("/sys/fs/bpf/sennet");
        if !pin_path.exists() {
            std::fs::create_dir_all(pin_path)?;
        }

        // Pin COUNTERS map
        tracing::info!("Pinning maps to /sys/fs/bpf/sennet...");
        if let Some(map) = bpf.map_mut("COUNTERS") {
            let _ = map.pin(pin_path.join("counters")); // Ignore if already pinned
        }
        
        // Pin DROP_EVENTS map (Phase 6.1)
        if let Some(map) = bpf.map_mut("DROP_EVENTS") {
            let _ = map.pin(pin_path.join("drop_events")); // Ignore if already pinned
        }

        // Attach TC Programs
        tracing::info!("Attaching TC classifiers to interface {}", interface);
        
        // Add clsact qdisc to the interface (ignore error if it already exists)
        let _ = tc::qdisc_add_clsact(interface);
        
        let ingress: &mut SchedClassifier = bpf.program_mut("tc_ingress").unwrap().try_into()?;
        ingress.load()?;
        ingress.attach(interface, TcAttachType::Ingress)?;

        let egress: &mut SchedClassifier = bpf.program_mut("tc_egress").unwrap().try_into()?;
        egress.load()?;
        egress.attach(interface, TcAttachType::Egress)?;

        // Try to attach kfree_skb tracepoint (Phase 6.1)
        // This may fail on older kernels or if tracepoint doesn't exist
        let mut drop_tracing_enabled = false;
        if let Some(prog) = bpf.program_mut("kfree_skb") {
            match prog.try_into() as Result<&mut TracePoint, _> {
                Ok(tp) => {
                    if let Err(e) = tp.load() {
                        tracing::warn!("Failed to load kfree_skb tracepoint: {}", e);
                    } else if let Err(e) = tp.attach("skb", "kfree_skb") {
                        tracing::warn!("Failed to attach kfree_skb tracepoint: {}", e);
                    } else {
                        tracing::info!("Attached kfree_skb tracepoint for drop reason tracing");
                        drop_tracing_enabled = true;
                    }
                }
                Err(e) => {
                    tracing::warn!("kfree_skb program not a tracepoint: {}", e);
                }
            }
        } else {
            tracing::debug!("kfree_skb program not found in eBPF binary");
        }

        // Try to attach nf_hook_slow tracepoint (Phase 6.2)
        let mut nf_tracing_enabled = false;
        if let Some(prog) = bpf.program_mut("nf_hook_slow") {
            match prog.try_into() as Result<&mut TracePoint, _> {
                Ok(tp) => {
                    if let Err(e) = tp.load() {
                        tracing::warn!("Failed to load nf_hook_slow tracepoint: {}", e);
                    } else if let Err(e) = tp.attach("netfilter", "nf_hook_slow") {
                        tracing::warn!("Failed to attach nf_hook_slow tracepoint: {}", e);
                    } else {
                        tracing::info!("Attached nf_hook_slow tracepoint for netfilter tracing");
                        nf_tracing_enabled = true;
                    }
                }
                Err(e) => {
                    tracing::warn!("nf_hook_slow program not a tracepoint: {}", e);
                }
            }
        } else {
            tracing::debug!("nf_hook_slow program not found in eBPF binary");
        }

        // Pin NF_EVENTS map if available
        if let Some(map) = bpf.map_mut("NF_EVENTS") {
            let _ = map.pin(pin_path.join("nf_events"));
        }

        Ok(Self {
            interface: interface.to_string(),
            bpf,
            drop_tracing_enabled,
            nf_tracing_enabled,
        })
    }

    /// Read current counters from eBPF maps
    #[cfg(target_os = "linux")]
    pub fn read_counters(&self) -> Result<PacketCounters> {
        let counters_map: PerCpuArray<_, PacketCounters> = 
            PerCpuArray::try_from(self.bpf.map("COUNTERS").unwrap())?;
        
        // Sum across all CPUs
        let mut total = PacketCounters::default();
        
        // Helper to sum counters for a given index
        let sum_values = |index: u32| -> Result<PacketCounters> {
            let values = counters_map.get(&index, 0)?;
            let mut sum = PacketCounters::default();
            for cpu_val in values.iter() {
                sum.rx_packets += cpu_val.rx_packets;
                sum.rx_bytes += cpu_val.rx_bytes;
                sum.tx_packets += cpu_val.tx_packets;
                sum.tx_bytes += cpu_val.tx_bytes;
                sum.drop_count += cpu_val.drop_count;
            }
            Ok(sum)
        };

        let ingress = sum_values(0)?;
        let egress = sum_values(1)?;

        total.rx_packets = ingress.rx_packets;
        total.rx_bytes = ingress.rx_bytes;
        total.tx_packets = egress.tx_packets;
        total.tx_bytes = egress.tx_bytes;
        total.drop_count = ingress.drop_count;

        Ok(total)
    }

    // Stub for non-Linux platforms
    #[cfg(not(target_os = "linux"))]
    pub fn load_and_attach(interface: &str) -> Result<Self> {
        tracing::warn!("eBPF not supported on this platform, using mock");
        Ok(Self {
            interface: interface.to_string(),
            drop_tracing_enabled: false,
            nf_tracing_enabled: false,
        })
    }

    #[cfg(not(target_os = "linux"))]
    pub fn read_counters(&self) -> Result<PacketCounters> {
        Ok(PacketCounters::default())
    }

    /// Get the attached interface name
    pub fn interface(&self) -> &str {
        &self.interface
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_packet_counters_default() {
        let counters = PacketCounters::default();
        assert_eq!(counters.rx_packets, 0);
        assert_eq!(counters.tx_packets, 0);
    }

    #[test]
    fn test_mock_manager() {
        let manager = EbpfManager::load_and_attach("lo").unwrap();
        assert_eq!(manager.interface(), "lo");
        let counters = manager.read_counters().unwrap();
        assert_eq!(counters.rx_packets, 0);
    }
}
