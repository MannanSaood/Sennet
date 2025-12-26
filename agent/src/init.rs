//! Interactive initialization wizard for Sennet Agent
//!
//! Provides a user-friendly setup experience for configuring the agent.

use anyhow::{Context, Result};
use colored::Colorize;
use std::io::{self, Write};
use std::path::PathBuf;
use std::fs;

/// Default config file path
#[cfg(unix)]
const DEFAULT_CONFIG_PATH: &str = "/etc/sennet/config.yaml";

#[cfg(not(unix))]
const DEFAULT_CONFIG_PATH: &str = "config.yaml";

/// Run the interactive initialization wizard
pub fn run() -> Result<()> {
    println!();
    println!("{}", "╔═══════════════════════════════════════╗".cyan());
    println!("{}", "║     Welcome to Sennet Setup Wizard    ║".cyan());
    println!("{}", "╚═══════════════════════════════════════╝".cyan());
    println!();

    // Get server URL
    let server_url = prompt_with_default(
        "Enter your Sennet server URL",
        "https://sennet.example.com",
    )?;

    // Validate URL format
    if !server_url.starts_with("http://") && !server_url.starts_with("https://") {
        println!("{}", "⚠ Warning: URL should start with http:// or https://".yellow());
    }

    // Get API key
    let api_key = prompt_required("Enter your API key (starts with sk_)")?;
    
    // Validate API key format
    if !api_key.starts_with("sk_") {
        println!("{}", "⚠ Warning: API key should start with 'sk_'".yellow());
    }

    // Optional: Network interface
    let interface = prompt_optional("Network interface to monitor (leave blank for auto-detect)")?;

    println!();
    println!("{}", "Testing connection...".dimmed());

    // Test connection (optional - just try to reach the server)
    match test_connection(&server_url, &api_key) {
        Ok(()) => println!("{}", "✓ Connection successful!".green()),
        Err(e) => {
            println!("{} {}", "⚠ Connection failed:".yellow(), e);
            println!("{}", "  Config will still be saved. Check your server is running.".dimmed());
        }
    }

    // Generate config content
    let config_content = generate_config(&server_url, &api_key, interface.as_deref());

    // Determine config path
    let config_path = PathBuf::from(DEFAULT_CONFIG_PATH);
    
    // Create parent directory if needed
    if let Some(parent) = config_path.parent() {
        if !parent.exists() {
            fs::create_dir_all(parent)
                .with_context(|| format!("Failed to create directory {:?}", parent))?;
        }
    }

    // Write config file
    fs::write(&config_path, &config_content)
        .with_context(|| format!("Failed to write config to {:?}", config_path))?;

    println!();
    println!("{}", "═══════════════════════════════════════".green());
    println!("{} {}", "✓ Configuration saved to:".green(), config_path.display());
    println!("{}", "═══════════════════════════════════════".green());
    println!();
    println!("Next steps:");
    println!("  1. Start the agent:  {}", "sudo systemctl start sennet".cyan());
    println!("  2. Check status:     {}", "sennet status".cyan());
    println!("  3. Monitor traffic:  {}", "sennet top".cyan());
    println!();

    Ok(())
}

/// Prompt for input with a default value
fn prompt_with_default(prompt: &str, default: &str) -> Result<String> {
    print!("{} [{}]: ", prompt, default.dimmed());
    io::stdout().flush()?;

    let mut input = String::new();
    io::stdin().read_line(&mut input)?;
    
    let input = input.trim();
    if input.is_empty() {
        Ok(default.to_string())
    } else {
        Ok(input.to_string())
    }
}

/// Prompt for required input
fn prompt_required(prompt: &str) -> Result<String> {
    loop {
        print!("{}: ", prompt);
        io::stdout().flush()?;

        let mut input = String::new();
        io::stdin().read_line(&mut input)?;
        
        let input = input.trim();
        if input.is_empty() {
            println!("{}", "This field is required.".red());
        } else {
            return Ok(input.to_string());
        }
    }
}

/// Prompt for optional input
fn prompt_optional(prompt: &str) -> Result<Option<String>> {
    print!("{}: ", prompt);
    io::stdout().flush()?;

    let mut input = String::new();
    io::stdin().read_line(&mut input)?;
    
    let input = input.trim();
    if input.is_empty() {
        Ok(None)
    } else {
        Ok(Some(input.to_string()))
    }
}

/// Test connection to the server
fn test_connection(server_url: &str, api_key: &str) -> Result<()> {
    let url = format!("{}/health", server_url.trim_end_matches('/'));
    
    let response = ureq::get(&url)
        .set("Authorization", &format!("Bearer {}", api_key))
        .timeout(std::time::Duration::from_secs(5))
        .call()
        .context("Failed to connect to server")?;

    if response.status() == 200 {
        Ok(())
    } else {
        anyhow::bail!("Server returned status {}", response.status())
    }
}

/// Generate YAML config content
fn generate_config(server_url: &str, api_key: &str, interface: Option<&str>) -> String {
    let mut config = format!(
r#"# Sennet Agent Configuration
# Generated by 'sennet init'

# API key for authentication with the control plane
api_key: {}

# URL of the Sennet control plane
server_url: {}

# Log level (trace, debug, info, warn, error)
log_level: info

# Heartbeat interval in seconds
heartbeat_interval_secs: 30
"#,
        api_key, server_url
    );

    if let Some(iface) = interface {
        config.push_str(&format!("\n# Network interface to monitor\ninterface: {}\n", iface));
    }

    config
}

/// Print help text for the init command
pub fn print_help() {
    println!("Initialize Sennet agent configuration");
    println!();
    println!("This wizard will guide you through setting up the agent:");
    println!("  - Server URL (your Sennet control plane)");
    println!("  - API key (from 'sennet-server keygen')");
    println!("  - Optional: specific network interface");
    println!();
    println!("The configuration will be saved to {}", DEFAULT_CONFIG_PATH);
}
