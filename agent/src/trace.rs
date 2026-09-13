//! Packet-event tracing is intentionally unavailable in the portable
//! counter-only collector. The CLI remains so existing invocations receive an
//! explicit error instead of fabricated or layout-dependent data.

use anyhow::Result;
use colored::Colorize;

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
                    if let Some(value) = args.get(i + 1) {
                        if let Some((ip, port)) = value.split_once(':') {
                            filter.dst_ip = Some(ip.to_string());
                            filter.dst_port = port.parse().ok();
                        } else {
                            filter.dst_ip = Some(value.clone());
                        }
                        i += 1;
                    }
                }
                "--src" => {
                    if let Some(value) = args.get(i + 1) {
                        if let Some((ip, port)) = value.split_once(':') {
                            filter.src_ip = Some(ip.to_string());
                            filter.src_port = port.parse().ok();
                        } else {
                            filter.src_ip = Some(value.clone());
                        }
                        i += 1;
                    }
                }
                "--proto" => {
                    if let Some(value) = args.get(i + 1) {
                        filter.protocol = Some(value.to_lowercase());
                        i += 1;
                    }
                }
                "--count" | "-c" => {
                    if let Some(value) = args.get(i + 1) {
                        filter.count = value.parse().unwrap_or(20);
                        i += 1;
                    }
                }
                "--timeout" | "-t" => {
                    if let Some(value) = args.get(i + 1) {
                        filter.timeout_secs = value.parse().unwrap_or(30);
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

pub fn run(args: &[String]) -> Result<()> {
    let _ = TraceFilter::parse(args)?;
    anyhow::bail!("packet-event tracing is unsupported by the portable counter-only collector; Sennet does not synthesize trace events")
}

pub fn print_help() {
    println!(
        "{}",
        "sennet trace - unsupported packet-event tracing".bold()
    );
    println!();
    println!("The portable collector reports TC packet/byte counters only.");
    println!("Drop events, process attribution, and packet-event tracing are unavailable.");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn trace_is_explicitly_unsupported() {
        let error = run(&[]).unwrap_err();
        assert!(error.to_string().contains("unsupported"));
        assert!(error.to_string().contains("does not synthesize"));
    }
}
