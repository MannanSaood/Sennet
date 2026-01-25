//! BTF (BPF Type Format) Support
//!
//! Provides utilities for checking kernel BTF support and handling
//! fallbacks for systems without BTF.

use std::path::Path;
use tracing::{info, warn};

/// BTF availability status
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BtfStatus {
    /// BTF is available at /sys/kernel/btf/vmlinux
    Available,
    /// BTF is not available, fallback required
    NotAvailable,
    /// Unknown (couldn't check)
    Unknown,
}

/// Check if the kernel has BTF support enabled
pub fn check_btf_support() -> BtfStatus {
    // Primary location for kernel BTF
    let btf_path = Path::new("/sys/kernel/btf/vmlinux");
    
    if btf_path.exists() {
        info!("BTF available at /sys/kernel/btf/vmlinux");
        return BtfStatus::Available;
    }
    
    // Check alternative locations
    let alt_paths = [
        "/boot/vmlinux",
        "/lib/modules", // Would need to check for BTF in module
    ];
    
    for path in alt_paths.iter() {
        if Path::new(path).exists() {
            info!("Potential BTF source at {}", path);
        }
    }
    
    warn!("BTF not available - eBPF CO-RE features will be limited");
    BtfStatus::NotAvailable
}

/// Check minimum kernel version for eBPF features
pub fn check_kernel_version() -> Option<(u32, u32, u32)> {
    #[cfg(target_os = "linux")]
    {
        use std::fs;
        
        if let Ok(release) = fs::read_to_string("/proc/sys/kernel/osrelease") {
            let parts: Vec<&str> = release.trim().split('.').collect();
            if parts.len() >= 2 {
                let major = parts[0].parse().ok()?;
                let minor = parts[1].split(|c: char| !c.is_numeric()).next()?.parse().ok()?;
                let patch = if parts.len() > 2 {
                    parts[2].split(|c: char| !c.is_numeric()).next()
                        .and_then(|s| s.parse().ok())
                        .unwrap_or(0)
                } else {
                    0
                };
                return Some((major, minor, patch));
            }
        }
        None
    }
    
    #[cfg(not(target_os = "linux"))]
    {
        None
    }
}

/// Check if kernel version meets minimum requirements (5.10+)
pub fn is_kernel_supported() -> bool {
    match check_kernel_version() {
        Some((major, minor, _)) => {
            if major > 5 || (major == 5 && minor >= 10) {
                info!("Kernel version {}.{} meets minimum requirements (5.10+)", major, minor);
                true
            } else {
                warn!("Kernel version {}.{} is below minimum (5.10)", major, minor);
                false
            }
        }
        None => {
            warn!("Could not determine kernel version");
            false
        }
    }
}

/// eBPF capability checks result
#[derive(Debug)]
pub struct EbpfCapabilities {
    pub btf_status: BtfStatus,
    pub kernel_version: Option<(u32, u32, u32)>,
    pub kernel_supported: bool,
    pub can_use_core: bool,
}

/// Check all eBPF capabilities
pub fn check_ebpf_capabilities() -> EbpfCapabilities {
    let btf_status = check_btf_support();
    let kernel_version = check_kernel_version();
    let kernel_supported = is_kernel_supported();
    
    // CO-RE requires both BTF and supported kernel
    let can_use_core = btf_status == BtfStatus::Available && kernel_supported;
    
    if can_use_core {
        info!("eBPF CO-RE: enabled (BTF available, kernel supported)");
    } else {
        warn!("eBPF CO-RE: disabled (using static offsets fallback)");
    }
    
    EbpfCapabilities {
        btf_status,
        kernel_version,
        kernel_supported,
        can_use_core,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_btf_check() {
        // Just ensure it doesn't panic
        let status = check_btf_support();
        println!("BTF Status: {:?}", status);
    }

    #[test]
    fn test_kernel_version() {
        let version = check_kernel_version();
        println!("Kernel version: {:?}", version);
    }

    #[test]
    fn test_capabilities() {
        let caps = check_ebpf_capabilities();
        println!("eBPF Capabilities: {:?}", caps);
    }
}
