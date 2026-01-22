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
mod k8s;
mod flows;

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
            "diagnose" => {
                // Kubernetes connectivity diagnosis (Phase 7.4)
                let diag_args: Vec<String> = args[2..].to_vec();
                if diag_args.iter().any(|a| a == "--help" || a == "-h") {
                    print_diagnose_help();
                } else {
                    run_diagnose(&diag_args).await?;
                }
                return Ok(());
            }
            "flows" => {
                // Network flow tracking with PID attribution (Phase 8)
                let flow_args: Vec<String> = args[2..].to_vec();
                if flow_args.iter().any(|a| a == "--help" || a == "-h") {
                    flows::print_help();
                } else {
                    flows::run(&flow_args)?;
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

    // Discover network interface (used by eBPF on Linux)
    #[allow(unused_variables)] // Used only on Linux for eBPF attachment
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
    println!("    {}       Active flows with PID attribution", "flows".cyan());
    println!("    {}    K8s pod connectivity diagnosis", "diagnose".cyan());
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
    println!("    sennet flows --pid 1234  # Show flows for process");
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

// =============================================================================
// Diagnose Command (Phase 7.4)
// =============================================================================

fn print_diagnose_help() {
    println!("{}", "Sennet Diagnose - Kubernetes Connectivity Diagnosis".bold());
    println!("Check connectivity between two pods and detect blocking NetworkPolicies");
    println!();
    println!("{}", "USAGE:".yellow());
    println!("    sennet diagnose <SOURCE_POD> <TARGET_POD> [OPTIONS]");
    println!();
    println!("{}", "OPTIONS:".yellow());
    println!("    -n, --namespace <NS>   Namespace (default: default)");
    println!("    -h, --help             Show this help message");
    println!();
    println!("{}", "EXAMPLES:".yellow());
    println!("    sennet diagnose frontend backend");
    println!("    sennet diagnose frontend backend -n production");
    println!("    sennet diagnose web-abc123 api-def456 --namespace staging");
    println!();
    println!("{}", "OUTPUT:".yellow());
    println!("    - Source and target pod details");
    println!("    - NetworkPolicies affecting each pod");
    println!("    - Connectivity status (ALLOWED / BLOCKED / UNKNOWN)");
    println!("    - Recommendations for troubleshooting");
    println!();
    println!("{}", "NOTES:".yellow());
    println!("    - Must be run from within a Kubernetes cluster");
    println!("    - Requires RBAC permissions to list pods and NetworkPolicies");
    println!("    - Works with standard K8s NetworkPolicy, Calico, and Cilium");
}

async fn run_diagnose(args: &[String]) -> Result<()> {
    // Parse arguments
    let mut source_pod: Option<String> = None;
    let mut target_pod: Option<String> = None;
    let mut namespace: Option<String> = None;
    
    let mut i = 0;
    while i < args.len() {
        let arg = &args[i];
        match arg.as_str() {
            "-n" | "--namespace" => {
                if i + 1 < args.len() {
                    namespace = Some(args[i + 1].clone());
                    i += 1;
                } else {
                    eprintln!("{} --namespace requires a value", "Error:".red());
                    std::process::exit(1);
                }
            }
            _ if !arg.starts_with('-') => {
                if source_pod.is_none() {
                    source_pod = Some(arg.clone());
                } else if target_pod.is_none() {
                    target_pod = Some(arg.clone());
                }
            }
            _ => {
                eprintln!("{} Unknown option: {}", "Error:".red(), arg);
                std::process::exit(1);
            }
        }
        i += 1;
    }
    
    // Validate required arguments
    let source = match source_pod {
        Some(s) => s,
        None => {
            eprintln!("{} Source pod name required", "Error:".red());
            eprintln!("Usage: sennet diagnose <SOURCE_POD> <TARGET_POD>");
            std::process::exit(1);
        }
    };
    
    let target = match target_pod {
        Some(t) => t,
        None => {
            eprintln!("{} Target pod name required", "Error:".red());
            eprintln!("Usage: sennet diagnose <SOURCE_POD> <TARGET_POD>");
            std::process::exit(1);
        }
    };
    
    info!("Diagnosing connectivity: {} -> {}", source, target);
    
    // Initialize K8s manager
    let k8s_manager = match k8s::K8sManager::new().await {
        Ok(mgr) => mgr,
        Err(e) => {
            eprintln!("{} Failed to initialize Kubernetes client: {}", "Error:".red(), e);
            std::process::exit(1);
        }
    };
    
    // Check if in cluster
    if !k8s_manager.is_in_cluster() {
        eprintln!("{} Not running inside a Kubernetes cluster", "Warning:".yellow());
        eprintln!("The diagnose command requires access to the Kubernetes API.");
        eprintln!();
        eprintln!("To use this command:");
        eprintln!("  1. Run sennet inside a Kubernetes pod with appropriate RBAC");
        eprintln!("  2. Or configure kubectl and set KUBECONFIG environment variable");
    }
    
    // Start sync to populate caches
    if let Err(e) = k8s_manager.start_sync().await {
        warn!("Failed to start K8s sync: {}", e);
    }
    
    // Give time for initial cache population
    tokio::time::sleep(std::time::Duration::from_millis(500)).await;
    
    // Run diagnosis
    match k8s_manager.diagnose_connectivity(&source, &target, namespace.as_deref()).await {
        Ok(result) => {
            println!("{}", result.format_output());
        }
        Err(e) => {
            eprintln!("{} Diagnosis failed: {}", "Error:".red(), e);
            std::process::exit(1);
        }
    }
    
    Ok(())
}
