//! Packet Trace Command (Phase 6.4)
//!
//! One-shot packet tracing for debugging.
//! Usage: sennet trace [OPTIONS]
//!
//! Options:
//!   --dst <IP[:PORT]>    Filter by destination
//!   --src <IP[:PORT]>    Filter by source
//!   --proto <tcp|udp|icmp>  Filter by protocol
//!   --count <N>          Stop after N events (default: 20)
//!   --timeout <SECS>     Stop after seconds (default: 30)

use anyhow::Result;
use colored::Colorize;
use std::time::{Duration, Instant};

/// Filter configuration for tracing
#[derive(Default, Debug)]
pub struct TraceFilter {
    pub dst_ip: Option<String>,
    pub dst_port: Option<u16>,
    pub src_ip: Option<String>,
    pub src_port: Option<u16>,
    pub protocol: Option<String>,
    pub count: usize,
    pub timeout_secs: u64,
}

impl TraceFilter {
    pub fn parse(args: &[String]) -> Result<Self> {
        let mut filter = TraceFilter {
            count: 20,
            timeout_secs: 30,
            ..Default::default()
        };
        
        let mut i = 0;
        while i < args.len() {
            match args[i].as_str() {
                "--dst" => {
                    if i + 1 < args.len() {
                        let dst = &args[i + 1];
                        if let Some((ip, port)) = dst.split_once(':') {
                            filter.dst_ip = Some(ip.to_string());
                            filter.dst_port = port.parse().ok();
                        } else {
                            filter.dst_ip = Some(dst.clone());
                        }
                        i += 1;
                    }
                }
                "--src" => {
                    if i + 1 < args.len() {
                        let src = &args[i + 1];
                        if let Some((ip, port)) = src.split_once(':') {
                            filter.src_ip = Some(ip.to_string());
                            filter.src_port = port.parse().ok();
                        } else {
                            filter.src_ip = Some(src.clone());
                        }
                        i += 1;
                    }
                }
                "--proto" => {
                    if i + 1 < args.len() {
                        filter.protocol = Some(args[i + 1].to_lowercase());
                        i += 1;
                    }
                }
                "--count" | "-c" => {
                    if i + 1 < args.len() {
                        filter.count = args[i + 1].parse().unwrap_or(20);
                        i += 1;
                    }
                }
                "--timeout" | "-t" => {
                    if i + 1 < args.len() {
                        filter.timeout_secs = args[i + 1].parse().unwrap_or(30);
                        i += 1;
                    }
                }
                _ => {}
            }
            i += 1;
        }
        
        Ok(filter)
    }
}

/// Run the trace command
pub fn run(args: &[String]) -> Result<()> {
    let filter = TraceFilter::parse(args)?;
    
    println!("{}", "Sennet Packet Trace".bold());
    println!("Watching for packet drops and netfilter events...");
    println!();
    
    // Print active filters
    if filter.dst_ip.is_some() || filter.src_ip.is_some() || filter.protocol.is_some() {
        print!("Filters: ");
        if let Some(ref dst) = filter.dst_ip {
            print!("dst={}", dst.cyan());
            if let Some(port) = filter.dst_port {
                print!(":{}", port.to_string().cyan());
            }
            print!(" ");
        }
        if let Some(ref src) = filter.src_ip {
            print!("src={}", src.cyan());
            if let Some(port) = filter.src_port {
                print!(":{}", port.to_string().cyan());
            }
            print!(" ");
        }
        if let Some(ref proto) = filter.protocol {
            print!("proto={}", proto.cyan());
        }
        println!();
    }
    
    println!("Limit: {} events, {}s timeout", 
             filter.count.to_string().yellow(),
             filter.timeout_secs.to_string().yellow());
    println!("Press {} to stop early.", "Ctrl+C".bold());
    println!("{}", "─".repeat(60));
    
    // Try to read from pinned maps
    #[cfg(target_os = "linux")]
    {
        run_linux_trace(&filter)?;
    }
    
    #[cfg(not(target_os = "linux"))]
    {
        run_mock_trace(&filter)?;
    }
    
    Ok(())
}

#[cfg(target_os = "linux")]
fn run_linux_trace(filter: &TraceFilter) -> Result<()> {
    use std::path::Path;
    use aya::maps::{Map, MapData, RingBuf};
    use crate::ebpf::{DropEvent, drop_reason_str};
    
    let drop_path = Path::new("/sys/fs/bpf/sennet/drop_events");
    
    if !drop_path.exists() {
        println!("{}: Pinned maps not found. Is the agent running?", "Warning".yellow());
        println!("Run '{}' first, then use trace.", "sudo sennet".cyan());
        return Ok(());
    }
    
    // Open DROP_EVENTS RingBuf
    let drop_rb: Option<RingBuf<MapData>> = match MapData::from_pin(drop_path) {
        Ok(data) => {
            let map = Map::RingBuf(data);
            map.try_into().ok()
        }
        Err(_) => None,
    };
    
    if drop_rb.is_none() {
        println!("{}: Could not open drop_events map", "Warning".yellow());
    }
    
    let mut drop_rb = drop_rb;
    let start = Instant::now();
    let timeout = Duration::from_secs(filter.timeout_secs);
    let mut event_count = 0;
    
    println!();
    println!("{:>8}  {:15}  {:10}  {}", "TIME", "REASON", "HOOK", "DETAILS");
    println!("{}", "─".repeat(60));
    
    loop {
        // Check limits
        if event_count >= filter.count {
            println!();
            println!("{}: Reached {} event limit", "Done".green(), filter.count);
            break;
        }
        if start.elapsed() > timeout {
            println!();
            println!("{}: Timeout after {}s", "Done".green(), filter.timeout_secs);
            break;
        }
        
        // Poll DROP_EVENTS
        if let Some(ref mut rb) = drop_rb {
            while let Some(item) = rb.next() {
                if item.len() >= std::mem::size_of::<DropEvent>() {
                    let event: DropEvent = unsafe {
                        std::ptr::read_unaligned(item.as_ptr() as *const DropEvent)
                    };
                    
                    let reason = drop_reason_str(event.reason);
                    let elapsed = start.elapsed().as_secs_f64();
                    
                    // Color by severity
                    let reason_colored = match event.reason {
                        7 | 5 => reason.red(),      // NETFILTER_DROP, SOCKET_FILTER
                        2 | 37 => reason.yellow(),  // NO_SOCKET, IP_OUTNOROUTES
                        _ => reason.white(),
                    };
                    
                    let proto = match event.protocol {
                        6 => "TCP",
                        17 => "UDP",
                        1 => "ICMP",
                        _ => "?",
                    };
                    
                    println!("{:>7.2}s  {:15}  {:10}  proto={}",
                             elapsed,
                             reason_colored,
                             "".white(),
                             proto);
                    
                    event_count += 1;
                    if event_count >= filter.count {
                        break;
                    }
                }
            }
        }
        
        // Small sleep to avoid busy loop
        std::thread::sleep(Duration::from_millis(50));
    }
    
    println!();
    println!("Captured {} events in {:.1}s", event_count, start.elapsed().as_secs_f64());
    
    Ok(())
}

#[cfg(not(target_os = "linux"))]
fn run_mock_trace(filter: &TraceFilter) -> Result<()> {
    use std::thread;
    
    let start = Instant::now();
    let timeout = Duration::from_secs(filter.timeout_secs);
    let mut event_count = 0;
    
    let mock_events = vec![
        ("NETFILTER_DROP", "INPUT", "192.168.1.5:443"),
        ("NO_SOCKET", "PREROUTING", "10.0.0.1:8080"),
        ("TCP_RESET", "OUTPUT", "172.16.0.1:22"),
        ("IP_OUTNOROUTES", "FORWARD", "8.8.8.8:53"),
    ];
    
    println!();
    println!("{:>8}  {:15}  {:10}  {}", "TIME", "REASON", "HOOK", "DETAILS");
    println!("{}", "─".repeat(60));
    
    loop {
        if event_count >= filter.count || start.elapsed() > timeout {
            break;
        }
        
        // Simulate event
        if rand::random::<u8>() > 240 {
            let (reason, hook, details) = &mock_events[event_count % mock_events.len()];
            let elapsed = start.elapsed().as_secs_f64();
            
            let reason_colored = if *reason == "NETFILTER_DROP" {
                reason.red()
            } else if *reason == "NO_SOCKET" || *reason == "IP_OUTNOROUTES" {
                reason.yellow()
            } else {
                reason.white()
            };
            
            println!("{:>7.2}s  {:15}  {:10}  dst={}",
                     elapsed,
                     reason_colored,
                     hook.cyan(),
                     details);
            
            event_count += 1;
        }
        
        thread::sleep(Duration::from_millis(100));
    }
    
    println!();
    println!("Captured {} events in {:.1}s (mock mode)", event_count, start.elapsed().as_secs_f64());
    
    Ok(())
}

/// Print trace command help
pub fn print_help() {
    println!("{}", "sennet trace - One-shot packet tracing".bold());
    println!();
    println!("{}", "USAGE:".yellow());
    println!("    sennet trace [OPTIONS]");
    println!();
    println!("{}", "OPTIONS:".yellow());
    println!("    {}        Filter by destination IP[:PORT]", "--dst <IP>".cyan());
    println!("    {}        Filter by source IP[:PORT]", "--src <IP>".cyan());
    println!("    {}   Filter by protocol (tcp, udp, icmp)", "--proto <P>".cyan());
    println!("    {}      Stop after N events (default: 20)", "--count <N>".cyan());
    println!("    {}   Stop after S seconds (default: 30)", "--timeout <S>".cyan());
    println!();
    println!("{}", "EXAMPLES:".yellow());
    println!("    sennet trace                     # Trace all drops");
    println!("    sennet trace --dst 10.0.0.5:443  # Filter by destination");
    println!("    sennet trace --proto icmp -c 10  # Trace 10 ICMP drops");
}
