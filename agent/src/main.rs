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

use anyhow::Result;
use tracing::{info, error, warn};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};
use tokio::signal;

use crate::config::Config;
use crate::identity::IdentityManager;
use crate::heartbeat::HeartbeatLoop;
use crate::client::SentinelClient;
use crate::upgrade::Updater;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize tracing
    init_tracing();

    // Check for CLI commands
    let args: Vec<String> = std::env::args().collect();
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
            "version" | "--version" | "-v" => {
                println!("sennet v{}", upgrade::CURRENT_VERSION);
                return Ok(());
            }
            "help" | "--help" | "-h" => {
                println!("Sennet Agent - Network Observability");
                println!();
                println!("USAGE:");
                println!("    sennet [COMMAND]");
                println!();
                println!("COMMANDS:");
                println!("    (none)     Run the agent");
                println!("    upgrade    Check for and install updates");
                println!("    version    Print version information");
                println!("    help       Print this help message");
                return Ok(());
            }
            _ => {
                error!("Unknown command: {}", args[1]);
                error!("Run 'sennet help' for usage");
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
    let _interface = match interface::discover_default_interface(config.interface.as_deref()) {
        Ok(iface) => {
            info!("Network interface: {}", iface);
            iface
        }
        Err(e) => {
            warn!("Interface discovery failed: {}. eBPF will be disabled.", e);
            String::new()
        }
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
