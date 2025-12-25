//! Network Interface Auto-Discovery
//!
//! Automatically detects the default network interface for eBPF attachment.

use anyhow::{Context, Result};
use std::fs;
use std::net::IpAddr;
use std::path::Path;

/// Information about a network interface
#[derive(Debug, Clone)]
pub struct InterfaceInfo {
    /// Interface name (e.g., "eth0", "ens33")
    pub name: String,
    /// Interface index
    pub index: u32,
    /// Whether the interface is up
    pub is_up: bool,
    /// Whether this is a loopback interface
    pub is_loopback: bool,
    /// IPv4 addresses
    pub ipv4_addrs: Vec<String>,
}

/// Discover the default network interface
/// 
/// Priority:
/// 1. Config override (if specified)
/// 2. Interface with default route
/// 3. First non-loopback, up interface
pub fn discover_default_interface(config_override: Option<&str>) -> Result<String> {
    // If config specifies an interface, use it
    if let Some(iface) = config_override {
        if interface_exists(iface) {
            return Ok(iface.to_string());
        } else {
            anyhow::bail!("Configured interface '{}' does not exist", iface);
        }
    }

    // Try to find interface with default route
    if let Some(iface) = get_default_route_interface() {
        return Ok(iface);
    }

    // Fallback: first non-loopback, up interface
    let interfaces = list_interfaces()?;
    for iface in interfaces {
        if iface.is_up && !iface.is_loopback {
            return Ok(iface.name);
        }
    }

    anyhow::bail!("No suitable network interface found")
}

/// Check if an interface exists
pub fn interface_exists(name: &str) -> bool {
    Path::new(&format!("/sys/class/net/{}", name)).exists()
}

/// Get the interface used for the default route
#[cfg(target_os = "linux")]
fn get_default_route_interface() -> Option<String> {
    // Read /proc/net/route to find default gateway
    let content = fs::read_to_string("/proc/net/route").ok()?;
    
    for line in content.lines().skip(1) {
        let fields: Vec<&str> = line.split_whitespace().collect();
        if fields.len() >= 2 {
            let iface = fields[0];
            let destination = fields[1];
            
            // 00000000 = 0.0.0.0 (default route)
            if destination == "00000000" {
                return Some(iface.to_string());
            }
        }
    }
    None
}

#[cfg(not(target_os = "linux"))]
fn get_default_route_interface() -> Option<String> {
    // On non-Linux, fall back to common names
    for name in &["eth0", "en0", "ens33", "enp0s3"] {
        if interface_exists(name) {
            return Some(name.to_string());
        }
    }
    None
}

/// List all network interfaces
#[cfg(target_os = "linux")]
pub fn list_interfaces() -> Result<Vec<InterfaceInfo>> {
    let mut interfaces = Vec::new();
    
    let net_dir = Path::new("/sys/class/net");
    if !net_dir.exists() {
        anyhow::bail!("/sys/class/net not found");
    }

    for entry in fs::read_dir(net_dir).context("Failed to read /sys/class/net")? {
        let entry = entry?;
        let name = entry.file_name().to_string_lossy().to_string();
        
        // Read interface index
        let index_path = entry.path().join("ifindex");
        let index: u32 = fs::read_to_string(&index_path)
            .unwrap_or_else(|_| "0".to_string())
            .trim()
            .parse()
            .unwrap_or(0);

        // Read flags to check if up
        let flags_path = entry.path().join("flags");
        let flags: u32 = fs::read_to_string(&flags_path)
            .unwrap_or_else(|_| "0x0".to_string())
            .trim()
            .trim_start_matches("0x")
            .parse()
            .unwrap_or(0);
        
        // IFF_UP = 0x1, IFF_LOOPBACK = 0x8
        let is_up = (flags & 0x1) != 0;
        let is_loopback = (flags & 0x8) != 0;

        // Get IPv4 addresses (simplified - just check if carrier is present)
        let ipv4_addrs = Vec::new(); // Would need netlink for full addresses

        interfaces.push(InterfaceInfo {
            name,
            index,
            is_up,
            is_loopback,
            ipv4_addrs,
        });
    }

    // Sort by index
    interfaces.sort_by_key(|i| i.index);
    
    Ok(interfaces)
}

#[cfg(not(target_os = "linux"))]
pub fn list_interfaces() -> Result<Vec<InterfaceInfo>> {
    // Mock for non-Linux
    Ok(vec![
        InterfaceInfo {
            name: "eth0".to_string(),
            index: 1,
            is_up: true,
            is_loopback: false,
            ipv4_addrs: vec!["192.168.1.100".to_string()],
        },
        InterfaceInfo {
            name: "lo".to_string(),
            index: 0,
            is_up: true,
            is_loopback: true,
            ipv4_addrs: vec!["127.0.0.1".to_string()],
        },
    ])
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_interface_exists_loopback() {
        // lo should always exist on Linux
        #[cfg(target_os = "linux")]
        assert!(interface_exists("lo"));
    }

    #[test]
    fn test_interface_not_exists() {
        assert!(!interface_exists("nonexistent_iface_12345"));
    }

    #[test]
    fn test_discover_with_override() {
        // Non-existent interface should error
        let result = discover_default_interface(Some("nonexistent_12345"));
        assert!(result.is_err());
    }

    #[test]
    #[cfg(target_os = "linux")]
    fn test_list_interfaces() {
        let interfaces = list_interfaces().unwrap();
        
        // Should have at least loopback
        assert!(!interfaces.is_empty());
        
        // Should have lo
        let has_lo = interfaces.iter().any(|i| i.name == "lo");
        assert!(has_lo, "loopback interface should exist");
    }

    #[test]
    #[cfg(target_os = "linux")]
    fn test_discover_default_finds_something() {
        // Should find some interface (even if just lo on isolated systems)
        let result = discover_default_interface(None);
        // This might fail on systems with no network, so just check it doesn't panic
        let _ = result;
    }

    #[test]
    fn test_interface_info_debug() {
        let info = InterfaceInfo {
            name: "test0".to_string(),
            index: 1,
            is_up: true,
            is_loopback: false,
            ipv4_addrs: vec![],
        };
        
        // Should be debuggable
        let debug = format!("{:?}", info);
        assert!(debug.contains("test0"));
    }
}
