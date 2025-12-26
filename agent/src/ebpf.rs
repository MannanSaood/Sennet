//! eBPF Loader and Manager
//!
//! Loads and manages the TC eBPF programs, reads counters and events.
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

#[cfg(target_os = "linux")]
use {
    aya::{
        programs::{tc, SchedClassifier, TcAttachType},
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
}

impl EbpfManager {
    /// Load and attach eBPF programs to the specified interface
    #[cfg(target_os = "linux")]
    pub fn load_and_attach(interface: &str) -> Result<Self> {
        tracing::info!("Loading eBPF programs...");
        
        // Load the eBPF binary
        // During CI/Cross build, build.rs copies binary to OUT_DIR/sennet_ebpf.bin
        #[cfg(feature = "embed_bpf")]
        let mut bpf = Bpf::load(include_bytes!(concat!(env!("OUT_DIR"), "/sennet_ebpf.bin")))?;
        
        #[cfg(not(feature = "embed_bpf"))]
        let mut bpf = Bpf::load(include_bytes!("../../sennet-ebpf/target/bpfel-unknown-none/release/sennet-ebpf"))?;
        
        // Pin path for maps
        let pin_path = Path::new("/sys/fs/bpf/sennet");
        if !pin_path.exists() {
            std::fs::create_dir_all(pin_path)?;
        }

        // Pin COUNTERS map using the Map trait's pin method
        tracing::info!("Pinning maps to /sys/fs/bpf/sennet...");
        if let Some(map) = bpf.map_mut("COUNTERS") {
            let _ = map.pin(pin_path.join("counters")); // Ignore if already pinned
        }
        
        // Note: RingBuf in aya 0.12 doesn't support pinning via this API
        // We skip pinning EVENTS for now - the map is still usable

        // Attach TC Programs
        tracing::info!("Attaching eBPF to interface {}", interface);
        
        // Add clsact qdisc to the interface (ignore error if it already exists)
        let _ = tc::qdisc_add_clsact(interface);
        
        let ingress: &mut SchedClassifier = bpf.program_mut("tc_ingress").unwrap().try_into()?;
        ingress.load()?;
        ingress.attach(interface, TcAttachType::Ingress)?;

        let egress: &mut SchedClassifier = bpf.program_mut("tc_egress").unwrap().try_into()?;
        egress.load()?;
        egress.attach(interface, TcAttachType::Egress)?;

        Ok(Self {
            interface: interface.to_string(),
            bpf,
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
            for cpu_val in values {
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
