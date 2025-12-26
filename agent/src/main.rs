//! Sennet Agent - Network Observability Agent
//!
//! This agent connects to the Sennet control plane, sends heartbeats,
//! and runs eBPF programs for packet analysis.

mod config;
mod identity;
mod heartbeat;
mod client;
mod interface;
mod ebpf;
mod upgrade;
mod status;
mod tui;
mod init;
mod trace;

use anyhow::Result;
use tracing::{info, error, warn};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};
use tokio::signal;
use colored::Colorize;

use crate::config::Config;
use crate::identity::IdentityManager;
use crate::heartbeat::HeartbeatLoop;
use crate::client::SentinelClient;
use crate::upgrade::Updater;

#[tokio::main]
async fn main() -> Result<()> {
    // Check for CLI commands first (before tracing init for cleaner output)
    let args: Vec<String> = std::env::args().collect();
    if args.len() > 1 {
        match args[1].as_str() {
            "init" => {
                // Init doesn't need tracing - it's interactive
                return init::run();
            }
            "help" | "--help" | "-h" => {
                print_help();
                return Ok(());
            }
            "version" | "--version" | "-v" => {
                println!("sennet v{}", upgrade::CURRENT_VERSION);
                return Ok(());
            }
            // Commands below need tracing
            _ => {}
        }
    }

    // Initialize tracing for remaining commands
    init_tracing();

    // Handle remaining commands
    if args.len() > 1 {
        match args[1].as_str() {
            "upgrade" => {
                info!("Checking for updates...");
                let updater = Updater::new()?;
                
                match updater.check_upgrade()? {
                    Some(version) => {
                        info!("New version available: v{}", version);
                        info!("Starting upgrade...");
                        updater.upgrade()?;
                        info!("Upgrade complete!");
                    }
                    None => {
                        info!("Already at latest version v{}", upgrade::CURRENT_VERSION);
                    }
                }
                return Ok(());
            }
            "status" => {
                status::run()?;
                return Ok(());
            }
            "top" => {
                tui::run()?;
                return Ok(());
            }
            "trace" => {
                // Pass remaining args to trace command
                let trace_args: Vec<String> = args[2..].to_vec();
                if trace_args.iter().any(|a| a == "--help" || a == "-h") {
                    trace::print_help();
                } else {
                    trace::run(&trace_args)?;
                }
                return Ok(());
            }
            cmd => {
                eprintln!("{} Unknown command: '{}'", "Error:".red(), cmd);
                eprintln!();
                eprintln!("Run '{}' for a list of available commands.", "sennet help".cyan());
                std::process::exit(1);
            }
        }
    }

    info!("Sennet Agent starting...");

    // Load configuration
    let config = match Config::load() {
        Ok(cfg) => {
            info!("Configuration loaded from {}", cfg.config_path().display());
            cfg
        }
        Err(e) => {
            error!("Failed to load configuration: {}", e);
            return Err(e);
        }
    };

    // Load or create agent identity
    let identity = match IdentityManager::load_or_create(&config) {
        Ok(id) => {
            info!("Agent ID: {}", id.agent_id());
            id
        }
        Err(e) => {
            error!("Failed to initialize identity: {}", e);
            return Err(e);
        }
    };

    // Discover network interface
    let interface = match interface::discover_default_interface(config.interface.as_deref()) {
        Ok(iface) => {
            info!("Network interface: {}", iface);
            iface
        }
        Err(e) => {
            warn!("Interface discovery failed: {}. eBPF will be disabled.", e);
            String::new()
        }
    };

    // Load and attach eBPF programs (Linux only)
    #[cfg(target_os = "linux")]
    let _ebpf_manager = if !interface.is_empty() {
        match ebpf::EbpfManager::load_and_attach(&interface) {
            Ok(mgr) => {
                info!("eBPF programs loaded successfully");
                if mgr.drop_tracing_enabled {
                    info!("Drop tracing: enabled (kfree_skb tracepoint attached)");
                }
                if mgr.nf_tracing_enabled {
                    info!("Netfilter tracing: enabled (nf_hook_slow tracepoint attached)");
                }
                Some(mgr)
            }
            Err(e) => {
                warn!("Failed to load eBPF programs: {}. Continuing without packet analysis.", e);
                None
            }
        }
    } else {
        None
    };

    // Create client
    let client = SentinelClient::new(&config)?;

    // Start heartbeat loop
    let heartbeat = HeartbeatLoop::new(config.clone(), identity, client);
    let heartbeat_handle = tokio::spawn(async move {
        if let Err(e) = heartbeat.run().await {
            error!("Heartbeat loop failed: {}", e);
        }
    });

    // Wait for shutdown signal
    info!("Agent running. Press Ctrl+C to stop.");
    shutdown_signal().await;

    // Graceful shutdown
    warn!("Shutdown signal received, stopping...");
    heartbeat_handle.abort();
    
    info!("Agent stopped");
    Ok(())
}

fn init_tracing() {
    let filter = EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| EnvFilter::new("info"));

    tracing_subscriber::registry()
        .with(filter)
        .with(tracing_subscriber::fmt::layer())
        .init();
}

fn print_help() {
    println!("{}", "Sennet Agent - Network Observability".bold());
    println!("High-performance network monitoring with eBPF");
    println!();
    println!("{}", "USAGE:".yellow());
    println!("    sennet [COMMAND]");
    println!();
    println!("{}", "COMMANDS:".yellow());
    println!("    {}       Run the agent daemon", "(none)".cyan());
    println!("    {}        Initialize configuration interactively", "init".cyan());
    println!("    {}      Display agent status and connection info", "status".cyan());
    println!("    {}         Live traffic monitoring dashboard", "top".cyan());
    println!("    {}       One-shot packet tracing", "trace".cyan());
    println!("    {}     Check for and install updates", "upgrade".cyan());
    println!("    {}     Print version information", "version".cyan());
    println!("    {}        Show this help message", "help".cyan());
    println!();
    println!("{}", "EXAMPLES:".yellow());
    println!("    sennet init              # Configure the agent");
    println!("    sudo sennet              # Run as daemon");
    println!("    sennet status            # Check agent status");
    println!("    sennet top               # Monitor traffic live");
    println!("    sennet trace --dst 10.0.0.5  # Trace drops to IP");
    println!();
    println!("{}", "CONFIGURATION:".yellow());
    println!("    Config file: /etc/sennet/config.yaml");
    println!("    Or use environment variables:");
    println!("      SENNET_API_KEY, SENNET_SERVER_URL");
    println!();
    println!("For more information, visit: https://github.com/MannanSaood/Sennet");
}

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("Failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("Failed to install SIGTERM handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }
}
