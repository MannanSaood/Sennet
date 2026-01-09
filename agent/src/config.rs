//! Configuration management for Sennet Agent
//!
//! Loads configuration from YAML file with environment variable overrides.

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};
use std::fs;

/// Agent configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    /// API key for authentication with the control plane
    pub api_key: String,

    /// URL of the Sennet control plane
    pub server_url: String,

    /// Log level (trace, debug, info, warn, error)
    #[serde(default = "default_log_level")]
    pub log_level: String,

    /// Network interface to monitor (None = auto-detect)
    #[serde(default)]
    pub interface: Option<String>,

    /// Heartbeat interval in seconds
    #[serde(default = "default_heartbeat_interval")]
    pub heartbeat_interval_secs: u64,

    /// Path to state directory
    #[serde(default = "default_state_dir")]
    pub state_dir: PathBuf,

    /// Path where config was loaded from (not serialized)
    #[serde(skip)]
    pub config_path: PathBuf,
}

fn default_log_level() -> String {
    "info".to_string()
}

fn default_heartbeat_interval() -> u64 {
    30
}

fn default_state_dir() -> PathBuf {
    if cfg!(unix) {
        PathBuf::from("/var/lib/sennet")
    } else {
        dirs::data_local_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join("sennet")
    }
}

impl Config {
    /// Load configuration from default locations or environment
    pub fn load() -> Result<Self> {
        // Check env vars first - takes priority
        if let (Ok(api_key), Ok(server_url)) = (
            std::env::var("SENNET_API_KEY"),
            std::env::var("SENNET_SERVER_URL"),
        ) {
            let config = Config {
                api_key,
                server_url,
                log_level: std::env::var("SENNET_LOG_LEVEL").unwrap_or_else(|_| default_log_level()),
                interface: std::env::var("SENNET_INTERFACE").ok(),
                heartbeat_interval_secs: std::env::var("SENNET_HEARTBEAT_INTERVAL")
                    .ok()
                    .and_then(|s| s.parse().ok())
                    .unwrap_or_else(default_heartbeat_interval),
                state_dir: default_state_dir(),
                config_path: PathBuf::from("env"),
            };
            config.validate()?;
            return Ok(config);
        }

        // Try config files
        let paths = Self::config_paths();
        
        for path in &paths {
            if path.exists() {
                return Self::load_from_file(path);
            }
        }

        anyhow::bail!(
            "No configuration found. Tried: {:?}\nOr set SENNET_API_KEY and SENNET_SERVER_URL environment variables.",
            paths
        );
    }

    /// Load configuration from a specific file
    pub fn load_from_file(path: &Path) -> Result<Self> {
        let content = fs::read_to_string(path)
            .with_context(|| format!("Failed to read config file: {}", path.display()))?;

        let mut config: Config = serde_yaml::from_str(&content)
            .with_context(|| format!("Failed to parse config file: {}", path.display()))?;

        config.config_path = path.to_path_buf();

        // Environment variables override file values
        if let Ok(api_key) = std::env::var("SENNET_API_KEY") {
            config.api_key = api_key;
        }
        if let Ok(server_url) = std::env::var("SENNET_SERVER_URL") {
            config.server_url = server_url;
        }
        if let Ok(log_level) = std::env::var("SENNET_LOG_LEVEL") {
            config.log_level = log_level;
        }
        if let Ok(interface) = std::env::var("SENNET_INTERFACE") {
            config.interface = Some(interface);
        }

        config.validate()?;
        Ok(config)
    }

    /// Get the path where config was loaded from
    pub fn config_path(&self) -> &Path {
        &self.config_path
    }

    /// Validate the configuration
    fn validate(&self) -> Result<()> {
        if self.api_key.is_empty() {
            anyhow::bail!("api_key cannot be empty");
        }
        if !self.api_key.starts_with("sk_") {
            anyhow::bail!("api_key must start with 'sk_'");
        }
        if self.server_url.is_empty() {
            anyhow::bail!("server_url cannot be empty");
        }
        if !self.server_url.starts_with("http://") && !self.server_url.starts_with("https://") {
            anyhow::bail!("server_url must start with http:// or https://");
        }
        Ok(())
    }

    /// Get list of config file paths to try
    fn config_paths() -> Vec<PathBuf> {
        let mut paths = Vec::new();

        // 1. Current directory
        paths.push(PathBuf::from("config.yaml"));
        paths.push(PathBuf::from("sennet.yaml"));

        // 2. User config directory
        if let Some(config_dir) = dirs::config_dir() {
            paths.push(config_dir.join("sennet").join("config.yaml"));
        }

        // 3. System config (Linux)
        #[cfg(unix)]
        paths.push(PathBuf::from("/etc/sennet/config.yaml"));

        // 4. Windows ProgramData
        #[cfg(windows)]
        if let Ok(program_data) = std::env::var("ProgramData") {
            paths.push(PathBuf::from(program_data).join("sennet").join("config.yaml"));
        }

        paths
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    fn create_test_config(dir: &TempDir, content: &str) -> PathBuf {
        let path = dir.path().join("config.yaml");
        fs::write(&path, content).unwrap();
        path
    }

    #[test]
    fn test_load_valid_config() {
        // Clear any env vars that might interfere
        std::env::remove_var("SENNET_API_KEY");
        std::env::remove_var("SENNET_SERVER_URL");
        
        let dir = TempDir::new().unwrap();
        let config_content = r#"
api_key: sk_test123456789
server_url: https://sennet.example.com
log_level: debug
"#;
        let path = create_test_config(&dir, config_content);
        
        let config = Config::load_from_file(&path).unwrap();
        
        assert_eq!(config.api_key, "sk_test123456789");
        assert_eq!(config.server_url, "https://sennet.example.com");
        assert_eq!(config.log_level, "debug");
        assert!(config.interface.is_none());
    }

    #[test]
    fn test_load_config_with_interface() {
        let dir = TempDir::new().unwrap();
        let config_content = r#"
api_key: sk_test123456789
server_url: https://sennet.example.com
interface: eth0
"#;
        let path = create_test_config(&dir, config_content);
        
        let config = Config::load_from_file(&path).unwrap();
        
        assert_eq!(config.interface, Some("eth0".to_string()));
    }

    #[test]
    fn test_invalid_api_key_prefix() {
        // Clear all env vars that could override
        std::env::remove_var("SENNET_API_KEY");
        std::env::remove_var("SENNET_SERVER_URL");
        
        let dir = TempDir::new().unwrap();
        let config_content = r#"
api_key: invalid_key
server_url: https://sennet.example.com
"#;
        let path = create_test_config(&dir, config_content);
        
        let result = Config::load_from_file(&path);
        assert!(result.is_err(), "Expected error for invalid api_key prefix");
        assert!(result.unwrap_err().to_string().contains("sk_"));
    }

    #[test]
    fn test_invalid_server_url() {
        let dir = TempDir::new().unwrap();
        let config_content = r#"
api_key: sk_test123456789
server_url: not-a-url
"#;
        let path = create_test_config(&dir, config_content);
        
        let result = Config::load_from_file(&path);
        assert!(result.is_err());
    }

    #[test]
    fn test_default_values() {
        let dir = TempDir::new().unwrap();
        let config_content = r#"
api_key: sk_test123456789
server_url: https://sennet.example.com
"#;
        let path = create_test_config(&dir, config_content);
        
        let config = Config::load_from_file(&path).unwrap();
        
        assert_eq!(config.log_level, "info");
        assert_eq!(config.heartbeat_interval_secs, 30);
    }

    // Note: Tests that use env vars can't run in parallel safely.
    // Run with: cargo test -- --test-threads=1
    // Or use unique test-specific env var names.
    #[test]
    #[ignore] // Ignored due to env var race conditions in parallel tests
    fn test_env_override() {
        let dir = TempDir::new().unwrap();
        let config_content = r#"
api_key: sk_file_key
server_url: https://file.example.com
"#;
        let path = create_test_config(&dir, config_content);
        
        std::env::set_var("SENNET_API_KEY", "sk_env_key");
        let config = Config::load_from_file(&path).unwrap();
        std::env::remove_var("SENNET_API_KEY");
        
        assert_eq!(config.api_key, "sk_env_key");
    }
}
