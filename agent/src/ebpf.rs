//! eBPF Loader and Manager
//!
//! Loads and manages the TC eBPF programs and kfree_skb tracepoint.
//! Reads counters and drop events.
//! On non-Linux platforms, provides a mock implementation.
//!
//! Note: Types mirror sennet-common for binary compatibility with eBPF programs.
//! These types are used by: heartbeat (metrics), tui (live display), trace (drop events).

use anyhow::Result;

/// Packet counters structure (mirrors eBPF side in sennet-common)
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

/// Drop event structure (mirrors eBPF side in sennet-common)
/// Used for kfree_skb tracepoint events
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
#[allow(dead_code)] // Used on Linux; exposed for cross-platform API consistency
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
#[allow(dead_code)] // Used on Linux
pub fn drop_reason_str(reason: u32) -> &'static str {
    match reason {
        0 => "NO_REASON",       // Kernel doesn't support drop reasons or couldn't read
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

/// Human-readable Ethernet protocol string
#[allow(dead_code)] // Used on Linux
pub fn eth_proto_str(proto: u16) -> &'static str {
    match proto {
        0x0800 => "IPv4",
        0x86DD => "IPv6",
        0x0806 => "ARP",
        0x8100 => "VLAN",
        _ => "OTHER",
    }
}

/// Netfilter event structure (mirrors eBPF side in sennet-common)
/// Used for nf_hook_slow tracepoint events (Phase 6.2)
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
#[allow(dead_code)] // Used on Linux
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

/// Human-readable hook name
#[allow(dead_code)] // Used on Linux
pub fn nf_hook_str(hook: u8) -> &'static str {
    match hook {
        0 => "PREROUTING",
        1 => "INPUT",
        2 => "FORWARD",
        3 => "OUTPUT",
        4 => "POSTROUTING",
        _ => "UNKNOWN",
    }
}

/// Human-readable verdict name
#[allow(dead_code)] // Used on Linux
pub fn nf_verdict_str(verdict: u8) -> &'static str {
    match verdict {
        0 => "DROP",
        1 => "ACCEPT",
        2 => "STOLEN",
        3 => "QUEUE",
        4 => "REPEAT",
        5 => "STOP",
        _ => "UNKNOWN",
    }
}

// ============================================================================
// Flow Tracking Types (Phase 8: Process Attribution)
// ============================================================================

/// 5-tuple flow key for tracking connections (mirrors eBPF side)
#[repr(C)]
#[derive(Clone, Copy, Default, Debug, PartialEq, Eq, Hash)]
#[allow(dead_code)]
pub struct FlowKey {
    pub src_ip: u32,
    pub dst_ip: u32,
    pub src_port: u16,
    pub dst_port: u16,
    pub protocol: u8,
    pub _pad: [u8; 3],
}

#[cfg(target_os = "linux")]
unsafe impl aya::Pod for FlowKey {}

/// Flow information with PID attribution (mirrors eBPF side)
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
#[allow(dead_code)]
pub struct FlowInfo {
    pub pid: u32,
    pub tgid: u32,
    pub comm: [u8; 16],
    pub start_time_ns: u64,
    pub rx_bytes: u64,
    pub tx_bytes: u64,
    pub rx_packets: u32,
    pub tx_packets: u32,
    pub state: u8,
    pub direction: u8,
    pub _pad: [u8; 2],
}

#[cfg(target_os = "linux")]
unsafe impl aya::Pod for FlowInfo {}

/// Flow event from RingBuf (mirrors eBPF side)
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
#[allow(dead_code)]
pub struct FlowEvent {
    pub timestamp_ns: u64,
    pub event_type: u8,
    pub direction: u8,
    pub protocol: u8,
    pub _pad: u8,
    pub pid: u32,
    pub src_ip: u32,
    pub dst_ip: u32,
    pub src_port: u16,
    pub dst_port: u16,
    pub comm: [u8; 16],
}

#[cfg(target_os = "linux")]
unsafe impl aya::Pod for FlowEvent {}

/// Human-readable flow direction
#[allow(dead_code)]
pub fn flow_direction_str(direction: u8) -> &'static str {
    match direction {
        1 => "OUT",
        2 => "IN",
        _ => "?",
    }
}

/// Human-readable flow event type
#[allow(dead_code)]
pub fn flow_event_type_str(event_type: u8) -> &'static str {
    match event_type {
        1 => "NEW",
        2 => "UPDATE",
        3 => "CLOSE",
        _ => "UNKNOWN",
    }
}

/// Convert comm bytes to string
#[allow(dead_code)]
pub fn comm_to_string(comm: &[u8; 16]) -> String {
    let end = comm.iter().position(|&c| c == 0).unwrap_or(16);
    String::from_utf8_lossy(&comm[..end]).to_string()
}

/// Format IP address from network byte order
#[allow(dead_code)]
pub fn format_ip(ip: u32) -> String {
    let bytes = ip.to_be_bytes();
    format!("{}.{}.{}.{}", bytes[0], bytes[1], bytes[2], bytes[3])
}

#[cfg(target_os = "linux")]
use {
    aya::{
        include_bytes_aligned,
        programs::{tc, SchedClassifier, TcAttachType, TracePoint, KProbe},
        maps::{PerCpuArray, lru_hash_map::LruHashMap},
        Bpf,
    },
    std::path::Path,
};

/// eBPF program manager
/// 
/// On Linux: Loads and attaches TC classifiers and tracepoints
/// On other platforms: Provides a mock implementation for development
#[allow(dead_code)] // Used only on Linux; mock on other platforms
pub struct EbpfManager {
    interface: String,
    #[cfg(target_os = "linux")]
    bpf: Bpf,
    /// Whether drop tracing is active (kfree_skb tracepoint attached)
    pub drop_tracing_enabled: bool,
    /// Whether netfilter tracing is active (nf_hook_slow tracepoint attached)
    pub nf_tracing_enabled: bool,
    /// Whether flow tracking is active (tcp_connect/inet_csk_accept kprobes attached) (Phase 8)
    pub flow_tracing_enabled: bool,
}

#[allow(dead_code)] // Methods used on Linux; mock impl on other platforms
impl EbpfManager {
    /// Load and attach eBPF programs to the specified interface
    #[cfg(target_os = "linux")]
    pub fn load_and_attach(interface: &str) -> Result<Self> {
        tracing::info!("Loading eBPF programs...");
        
        // Load the eBPF binary with proper alignment for ELF parsing
        // NOTE: Must use include_bytes_aligned! instead of include_bytes! because
        // the ELF parser requires 8-byte aligned memory, which include_bytes! doesn't guarantee
        #[cfg(feature = "embed_bpf")]
        let ebpf_bytes: &[u8] = include_bytes_aligned!(concat!(env!("OUT_DIR"), "/sennet_ebpf.bin"));
        
        #[cfg(not(feature = "embed_bpf"))]
        let ebpf_bytes: &[u8] = include_bytes_aligned!("../sennet-ebpf/target/bpfel-unknown-none/release/sennet-ebpf");
        
        // Debug: Log embedded binary info
        tracing::info!("eBPF binary size: {} bytes", ebpf_bytes.len());
        if ebpf_bytes.len() >= 4 {
            tracing::info!(
                "eBPF ELF magic: {:02x} {:02x} {:02x} {:02x} (expected: 7f 45 4c 46 = ELF)",
                ebpf_bytes[0], ebpf_bytes[1], ebpf_bytes[2], ebpf_bytes[3]
            );
        }
        
        // Comprehensive ELF64 header inspection
        if ebpf_bytes.len() >= 64 {
            // ELF64 header fields (all offsets for 64-bit ELF)
            let ei_class = ebpf_bytes[4];  // 1=32-bit, 2=64-bit
            let ei_data = ebpf_bytes[5];   // 1=LE, 2=BE
            let ei_version = ebpf_bytes[6];
            let ei_osabi = ebpf_bytes[7];
            
            let e_type = u16::from_le_bytes([ebpf_bytes[16], ebpf_bytes[17]]);
            let e_machine = u16::from_le_bytes([ebpf_bytes[18], ebpf_bytes[19]]);
            let e_version = u32::from_le_bytes([ebpf_bytes[20], ebpf_bytes[21], ebpf_bytes[22], ebpf_bytes[23]]);
            
            // Key fields for alignment validation
            let e_ehsize = u16::from_le_bytes([ebpf_bytes[52], ebpf_bytes[53]]);  // ELF header size
            let e_phentsize = u16::from_le_bytes([ebpf_bytes[54], ebpf_bytes[55]]); // Program header entry size
            let e_phnum = u16::from_le_bytes([ebpf_bytes[56], ebpf_bytes[57]]);     // Number of program headers
            let e_shentsize = u16::from_le_bytes([ebpf_bytes[58], ebpf_bytes[59]]); // Section header entry size
            let e_shnum = u16::from_le_bytes([ebpf_bytes[60], ebpf_bytes[61]]);     // Number of section headers
            let e_shstrndx = u16::from_le_bytes([ebpf_bytes[62], ebpf_bytes[63]]);  // Section name string table index
            
            // Section header offset (8 bytes at offset 40)
            let e_shoff = u64::from_le_bytes([
                ebpf_bytes[40], ebpf_bytes[41], ebpf_bytes[42], ebpf_bytes[43],
                ebpf_bytes[44], ebpf_bytes[45], ebpf_bytes[46], ebpf_bytes[47]
            ]);
            
            tracing::info!("=== ELF64 Header Inspection ===");
            tracing::info!("EI_CLASS: {} (expected 2 for 64-bit)", ei_class);
            tracing::info!("EI_DATA: {} (expected 1 for little-endian)", ei_data);
            tracing::info!("EI_VERSION: {}", ei_version);
            tracing::info!("EI_OSABI: {}", ei_osabi);
            tracing::info!("e_type: {} (1=REL, 2=EXEC, 3=DYN)", e_type);
            tracing::info!("e_machine: {} (expected 247 for eBPF)", e_machine);
            tracing::info!("e_version: {}", e_version);
            tracing::info!("e_ehsize: {} (expected 64 for ELF64)", e_ehsize);
            tracing::info!("e_phentsize: {}", e_phentsize);
            tracing::info!("e_phnum: {}", e_phnum);
            tracing::info!("e_shentsize: {} (expected 64 for ELF64)", e_shentsize);
            tracing::info!("e_shnum: {}", e_shnum);
            tracing::info!("e_shstrndx: {}", e_shstrndx);
            tracing::info!("e_shoff: {} (section headers at byte offset)", e_shoff);
            
            // Validate critical alignment requirements
            if e_ehsize != 64 {
                tracing::error!("INVALID: ELF header size {} != 64", e_ehsize);
            }
            if e_shentsize != 64 && e_shentsize != 0 {
                tracing::error!("INVALID: Section header entry size {} != 64", e_shentsize);
            }
            if e_shoff as usize > ebpf_bytes.len() {
                tracing::error!("INVALID: Section header offset {} > file size {}", e_shoff, ebpf_bytes.len());
            }
            if e_shoff % 8 != 0 {
                tracing::error!("INVALID: Section header offset {} not 8-byte aligned", e_shoff);
            }
            if e_machine != 247 {
                tracing::error!("INVALID: e_machine {} is not eBPF (247)", e_machine);
            }
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

        // Try to attach flow tracking kprobes (Phase 8)
        let mut flow_tracing_enabled = false;
        
        // tcp_connect kprobe - track outbound connections
        if let Some(prog) = bpf.program_mut("tcp_connect") {
            match prog.try_into() as Result<&mut KProbe, _> {
                Ok(kp) => {
                    if let Err(e) = kp.load() {
                        tracing::warn!("Failed to load tcp_connect kprobe: {}", e);
                    } else if let Err(e) = kp.attach("tcp_connect", 0) {
                        tracing::warn!("Failed to attach tcp_connect kprobe: {}", e);
                    } else {
                        tracing::info!("Attached tcp_connect kprobe for outbound flow tracking");
                        flow_tracing_enabled = true;
                    }
                }
                Err(e) => {
                    tracing::warn!("tcp_connect program not a kprobe: {}", e);
                }
            }
        }
        
        // inet_csk_accept kprobe - track inbound connections
        if let Some(prog) = bpf.program_mut("inet_csk_accept") {
            match prog.try_into() as Result<&mut KProbe, _> {
                Ok(kp) => {
                    if let Err(e) = kp.load() {
                        tracing::warn!("Failed to load inet_csk_accept kprobe: {}", e);
                    } else if let Err(e) = kp.attach("inet_csk_accept", 0) {
                        tracing::warn!("Failed to attach inet_csk_accept kprobe: {}", e);
                    } else {
                        tracing::info!("Attached inet_csk_accept kprobe for inbound flow tracking");
                    }
                }
                Err(e) => {
                    tracing::warn!("inet_csk_accept program not a kprobe: {}", e);
                }
            }
        }
        
        // tcp_close kprobe - track connection closures
        if let Some(prog) = bpf.program_mut("tcp_close") {
            match prog.try_into() as Result<&mut KProbe, _> {
                Ok(kp) => {
                    if let Err(e) = kp.load() {
                        tracing::warn!("Failed to load tcp_close kprobe: {}", e);
                    } else if let Err(e) = kp.attach("tcp_close", 0) {
                        tracing::warn!("Failed to attach tcp_close kprobe: {}", e);
                    } else {
                        tracing::info!("Attached tcp_close kprobe for flow cleanup");
                    }
                }
                Err(e) => {
                    tracing::warn!("tcp_close program not a kprobe: {}", e);
                }
            }
        }
        
        // Pin FLOWS map if available
        if let Some(map) = bpf.map_mut("FLOWS") {
            let _ = map.pin(pin_path.join("flows"));
        }
        
        // Pin FLOW_EVENTS map if available
        if let Some(map) = bpf.map_mut("FLOW_EVENTS") {
            let _ = map.pin(pin_path.join("flow_events"));
        }

        Ok(Self {
            interface: interface.to_string(),
            bpf,
            drop_tracing_enabled,
            nf_tracing_enabled,
            flow_tracing_enabled,
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

    /// Read all active flows from eBPF LRU HashMap (Phase 8)
    #[cfg(target_os = "linux")]
    pub fn read_flows(&self) -> Result<Vec<(FlowKey, FlowInfo)>> {
        let flows_map: LruHashMap<_, FlowKey, FlowInfo> = 
            LruHashMap::try_from(self.bpf.map("FLOWS").ok_or_else(|| anyhow::anyhow!("FLOWS map not found"))?)?;
        
        let mut flows = Vec::new();
        for item in flows_map.iter() {
            if let Ok((key, value)) = item {
                flows.push((key, value));
            }
        }
        
        Ok(flows)
    }

    // Stub for non-Linux platforms
    #[cfg(not(target_os = "linux"))]
    pub fn load_and_attach(interface: &str) -> Result<Self> {
        tracing::warn!("eBPF not supported on this platform, using mock");
        Ok(Self {
            interface: interface.to_string(),
            drop_tracing_enabled: false,
            nf_tracing_enabled: false,
            flow_tracing_enabled: false,
        })
    }

    #[cfg(not(target_os = "linux"))]
    pub fn read_counters(&self) -> Result<PacketCounters> {
        Ok(PacketCounters::default())
    }

    #[cfg(not(target_os = "linux"))]
    pub fn read_flows(&self) -> Result<Vec<(FlowKey, FlowInfo)>> {
        Ok(Vec::new())
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
    fn test_drop_reason_str() {
        assert_eq!(drop_reason_str(7), "NETFILTER_DROP");
        assert_eq!(drop_reason_str(2), "NO_SOCKET");
        assert_eq!(drop_reason_str(999), "UNKNOWN");
    }

    #[test]
    fn test_nf_hook_str() {
        assert_eq!(nf_hook_str(0), "PREROUTING");
        assert_eq!(nf_hook_str(1), "INPUT");
        assert_eq!(nf_hook_str(4), "POSTROUTING");
    }

    #[test]
    fn test_nf_verdict_str() {
        assert_eq!(nf_verdict_str(0), "DROP");
        assert_eq!(nf_verdict_str(1), "ACCEPT");
    }

    // This test only works on non-Linux (mock mode) or requires root on Linux
    #[test]
    #[cfg(not(target_os = "linux"))]
    fn test_mock_manager() {
        let manager = EbpfManager::load_and_attach("lo").unwrap();
        assert_eq!(manager.interface(), "lo");
        let counters = manager.read_counters().unwrap();
        assert_eq!(counters.rx_packets, 0);
    }
}
