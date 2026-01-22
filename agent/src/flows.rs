//! Flow Tracking CLI Command (Phase 8)
//!
//! Displays active network flows with PID attribution.
//! Usage: sennet flows [OPTIONS]

use anyhow::Result;
use colored::Colorize;
use crate::ebpf::{EbpfManager, format_ip, comm_to_string, flow_direction_str};

/// Print help for the flows command
pub fn print_help() {
    println!("{}", "Sennet Flows - Active Network Flows with PID Attribution".bold());
    println!("Show all active TCP connections with process information.");
    println!();
    println!("{}", "USAGE:".yellow());
    println!("    sennet flows [OPTIONS]");
    println!();
    println!("{}", "OPTIONS:".yellow());
    println!("    --sort <FIELD>     Sort by: pid, bytes, packets (default: bytes)");
    println!("    --limit <N>        Show only top N flows (default: 50)");
    println!("    --pid <PID>        Filter by process ID");
    println!("    --comm <NAME>      Filter by process name (partial match)");
    println!("    -h, --help         Show this help message");
    println!();
    println!("{}", "EXAMPLES:".yellow());
    println!("    sennet flows                  # Show all flows");
    println!("    sennet flows --sort packets   # Sort by packet count");
    println!("    sennet flows --pid 1234       # Show flows for PID 1234");
    println!("    sennet flows --comm nginx     # Show flows for nginx");
    println!();
    println!("{}", "OUTPUT:".yellow());
    println!("    PID       Process name");
    println!("    DIR       Direction (IN=inbound, OUT=outbound)");
    println!("    LOCAL     Local IP:port");
    println!("    REMOTE    Remote IP:port");
    println!("    RX        Bytes received");
    println!("    TX        Bytes transmitted");
    println!();
    println!("{}", "NOTES:".yellow());
    println!("    - Requires root privileges for eBPF access");
    println!("    - Flow tracking must be enabled (kprobes attached)");
}

/// Sort field for flows
#[derive(Debug, Clone, Copy)]
pub enum SortField {
    Pid,
    Bytes,
    Packets,
}

/// Options for the flows command
pub struct FlowsOptions {
    pub sort_by: SortField,
    pub limit: usize,
    pub filter_pid: Option<u32>,
    pub filter_comm: Option<String>,
}

impl Default for FlowsOptions {
    fn default() -> Self {
        Self {
            sort_by: SortField::Bytes,
            limit: 50,
            filter_pid: None,
            filter_comm: None,
        }
    }
}

/// Parse command line arguments for flows command
pub fn parse_args(args: &[String]) -> FlowsOptions {
    let mut opts = FlowsOptions::default();
    let mut i = 0;
    
    while i < args.len() {
        match args[i].as_str() {
            "--sort" => {
                if i + 1 < args.len() {
                    opts.sort_by = match args[i + 1].as_str() {
                        "pid" => SortField::Pid,
                        "packets" => SortField::Packets,
                        _ => SortField::Bytes,
                    };
                    i += 1;
                }
            }
            "--limit" => {
                if i + 1 < args.len() {
                    opts.limit = args[i + 1].parse().unwrap_or(50);
                    i += 1;
                }
            }
            "--pid" => {
                if i + 1 < args.len() {
                    opts.filter_pid = args[i + 1].parse().ok();
                    i += 1;
                }
            }
            "--comm" => {
                if i + 1 < args.len() {
                    opts.filter_comm = Some(args[i + 1].clone());
                    i += 1;
                }
            }
            _ => {}
        }
        i += 1;
    }
    
    opts
}

/// Format bytes in human-readable form
fn format_bytes(bytes: u64) -> String {
    if bytes >= 1_000_000_000 {
        format!("{:.1}GB", bytes as f64 / 1_000_000_000.0)
    } else if bytes >= 1_000_000 {
        format!("{:.1}MB", bytes as f64 / 1_000_000.0)
    } else if bytes >= 1_000 {
        format!("{:.1}KB", bytes as f64 / 1_000.0)
    } else {
        format!("{}B", bytes)
    }
}

/// Run the flows command
pub fn run(args: &[String]) -> Result<()> {
    let opts = parse_args(args);
    
    // Discover interface and load eBPF
    let interface = crate::interface::discover_default_interface(None)?;
    let manager = EbpfManager::load_and_attach(&interface)?;
    
    if !manager.flow_tracing_enabled {
        eprintln!("{} Flow tracing not enabled. kprobes may have failed to attach.", "Warning:".yellow());
        eprintln!("This requires a recent kernel with kprobe support.");
    }
    
    // Read flows
    let mut flows = manager.read_flows()?;
    
    if flows.is_empty() {
        println!("{}", "No active flows found.".yellow());
        println!();
        println!("Possible reasons:");
        println!("  - No active TCP connections");
        println!("  - Flow tracking kprobes not attached");
        println!("  - Flows started before sennet was running");
        return Ok(());
    }
    
    // Apply filters
    if let Some(pid) = opts.filter_pid {
        flows.retain(|(_, info)| info.pid == pid);
    }
    if let Some(ref comm) = opts.filter_comm {
        let comm_lower = comm.to_lowercase();
        flows.retain(|(_, info)| {
            comm_to_string(&info.comm).to_lowercase().contains(&comm_lower)
        });
    }
    
    // Sort flows
    match opts.sort_by {
        SortField::Pid => flows.sort_by_key(|(_, info)| info.pid),
        SortField::Bytes => flows.sort_by_key(|(_, info)| std::cmp::Reverse(info.rx_bytes + info.tx_bytes)),
        SortField::Packets => flows.sort_by_key(|(_, info)| std::cmp::Reverse(info.rx_packets + info.tx_packets)),
    }
    
    // Limit
    flows.truncate(opts.limit);
    
    // Print header
    println!();
    println!("{}", "Sennet Active Flows".bold());
    println!("{}", "═".repeat(100));
    println!(
        "{:>7} {:>16} {:>3} {:>21} {:>21} {:>10} {:>10}",
        "PID".cyan(),
        "COMMAND".cyan(),
        "DIR".cyan(),
        "LOCAL".cyan(),
        "REMOTE".cyan(),
        "RX".cyan(),
        "TX".cyan()
    );
    println!("{}", "─".repeat(100));
    
    // Print flows
    for (key, info) in &flows {
        let comm = comm_to_string(&info.comm);
        let _direction = flow_direction_str(info.direction);
        
        // Format addresses based on direction
        let (local, remote) = if info.direction == 1 {
            // Outbound: src is local
            (
                format!("{}:{}", format_ip(key.src_ip), key.src_port),
                format!("{}:{}", format_ip(key.dst_ip), key.dst_port),
            )
        } else {
            // Inbound: dst is local
            (
                format!("{}:{}", format_ip(key.dst_ip), key.dst_port),
                format!("{}:{}", format_ip(key.src_ip), key.src_port),
            )
        };
        
        let dir_colored = if info.direction == 1 {
            "OUT".green()
        } else {
            "IN".blue()
        };
        
        println!(
            "{:>7} {:>16} {:>3} {:>21} {:>21} {:>10} {:>10}",
            info.pid,
            if comm.len() > 16 { &comm[..16] } else { &comm },
            dir_colored,
            local,
            remote,
            format_bytes(info.rx_bytes),
            format_bytes(info.tx_bytes),
        );
    }
    
    println!("{}", "─".repeat(100));
    println!("Total: {} flows", flows.len());
    println!();
    
    Ok(())
}
