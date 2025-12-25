//! eBPF Loader and Manager
//!
//! Loads and manages the TC eBPF programs, reads counters and events.
//! On non-Linux platforms, provides a mock implementation.

use anyhow::Result;

/// Packet counters structure (mirrors eBPF side)
#[repr(C)]
#[derive(Clone, Copy, Default, Debug)]
pub struct PacketCounters {
    pub rx_packets: u64,
    pub rx_bytes: u64,
    pub tx_packets: u64,
    pub tx_bytes: u64,
    pub drop_count: u64,
}

#[cfg(target_os = "linux")]
use {
    aya::{
        programs::{tc, SchedClassifier, TcAttachType},
        maps::{PerCpuArray, RingBuf, MapData},
        Bpf,
    },
    std::path::Path,
};

// ... (PacketCounters struct remains same)

/// eBPF program manager
pub struct EbpfManager {
    interface: String,
    #[cfg(target_os = "linux")]
    bpf: Bpf,
}

impl EbpfManager {
    /// Load and attach eBPF programs to the specified interface
    #[cfg(target_os = "linux")]
    pub fn load_and_attach(interface: &str) -> Result<Self> {
        tracing::info!("Loading eBPF programs...");
        
        // Load the eBPF binary (must be built previously)
        // Load the eBPF binary
        // During CI/Cross build, we copy the binary to agent/src/sennet-ebpf
        #[cfg(feature = "embed_bpf")]
        let mut bpf = Bpf::load(include_bytes!("sennet-ebpf"))?;
        
        #[cfg(not(feature = "embed_bpf"))]
        let mut bpf = Bpf::load(include_bytes!("../../sennet-ebpf/target/bpfel-unknown-none/release/sennet-ebpf"))?;
        
        // Pin path for maps
        let pin_path = Path::new("/sys/fs/bpf/sennet");
        if !pin_path.exists() {
            std::fs::create_dir_all(pin_path)?;
        }

        // Load and Pin Maps
        tracing::info!("Pinning maps to /sys/fs/bpf/sennet...");
        
        // Pin COUNTERS
        let mut counters: PerCpuArray<_, u32> = PerCpuArray::try_from(bpf.map_mut("COUNTERS").unwrap())?;
        counters.pin(pin_path.join("counters")).ok(); // Ignore if already pinned

        // Pin EVENTS
        let mut events: RingBuf<_> = RingBuf::try_from(bpf.map_mut("EVENTS").unwrap())?;
        events.pin(pin_path.join("events")).ok();

        // Attach TC Programs
        tracing::info!("Attaching eBPF to interface {}", interface);
        let _ = tc::qdisc::add_clsact(interface); // Ignore error if qdisc exists
        
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
        let counters_map: PerCpuArray<_, PacketCounters> = PerCpuArray::try_from(self.bpf.map("COUNTERS").unwrap())?;
        
        // Sum across all CPUs
        let mut total = PacketCounters::default();
        // Since PerCpuArray iteration is complex without proper helpers, 
        // and we are running in same process, we can just grab index 0 and 1.
        // Wait, PerCpuArray.get(index, flags) returns a generic `Values` which is Vec<T> per cpu.
        
        // Helper to sum counters
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
        total.drop_count = ingress.drop_count; // Drops only tracked on ingress currently logic wise

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
