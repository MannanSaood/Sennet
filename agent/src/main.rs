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

use anyhow::Result;
use tracing::{info, error, warn};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt, EnvFilter};
use tokio::signal;

use crate::config::Config;
use crate::identity::IdentityManager;
use crate::heartbeat::HeartbeatLoop;
use crate::client::SentinelClient;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize tracing
    init_tracing();

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
