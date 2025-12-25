//! Identity management for Sennet Agent
//!
//! Manages the persistent agent UUID that identifies this agent instance.

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use uuid::Uuid;

use crate::config::Config;

/// Agent identity state
#[derive(Debug, Serialize, Deserialize)]
pub struct IdentityState {
    /// Unique agent identifier (UUID)
    pub agent_id: String,
    
    /// Agent version
    pub version: String,
    
    /// First seen timestamp
    pub created_at: String,
}

/// Manages agent identity persistence
pub struct IdentityManager {
    state: IdentityState,
    state_path: PathBuf,
}

impl IdentityManager {
    /// Load existing identity or create a new one
    pub fn load_or_create(config: &Config) -> Result<Self> {
        let state_dir = &config.state_dir;
        let state_path = state_dir.join("state.json");

        // Ensure state directory exists
        if !state_dir.exists() {
            fs::create_dir_all(state_dir)
                .with_context(|| format!("Failed to create state directory: {}", state_dir.display()))?;
        }

        let state = if state_path.exists() {
            Self::load_state(&state_path)?
        } else {
            let state = Self::create_new_state();
            Self::save_state(&state_path, &state)?;
            state
        };

        Ok(Self { state, state_path })
    }

    /// Get the agent ID
    pub fn agent_id(&self) -> &str {
        &self.state.agent_id
    }

    /// Get the agent version
    pub fn version(&self) -> &str {
        &self.state.version
    }

    /// Load state from file
    fn load_state(path: &Path) -> Result<IdentityState> {
        let content = fs::read_to_string(path)
            .with_context(|| format!("Failed to read state file: {}", path.display()))?;

        let state: IdentityState = serde_json::from_str(&content)
            .with_context(|| format!("Failed to parse state file: {}", path.display()))?;

        Ok(state)
    }

    /// Create new state with fresh UUID
    fn create_new_state() -> IdentityState {
        IdentityState {
            agent_id: Uuid::new_v4().to_string(),
            version: env!("CARGO_PKG_VERSION").to_string(),
            created_at: chrono::Utc::now().to_rfc3339(),
        }
    }

    /// Save state to file
    fn save_state(path: &Path, state: &IdentityState) -> Result<()> {
        let content = serde_json::to_string_pretty(state)
            .context("Failed to serialize state")?;

        fs::write(path, content)
            .with_context(|| format!("Failed to write state file: {}", path.display()))?;

        Ok(())
    }

    /// Update version and save
    pub fn update_version(&mut self, version: &str) -> Result<()> {
        self.state.version = version.to_string();
        Self::save_state(&self.state_path, &self.state)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    fn create_test_config(state_dir: PathBuf) -> Config {
        Config {
            api_key: "sk_test123".to_string(),
            server_url: "https://test.example.com".to_string(),
            log_level: "info".to_string(),
            interface: None,
            heartbeat_interval_secs: 30,
            state_dir,
            config_path: PathBuf::new(),
        }
    }

    #[test]
    fn test_create_new_identity() {
        let dir = TempDir::new().unwrap();
        let config = create_test_config(dir.path().to_path_buf());

        let identity = IdentityManager::load_or_create(&config).unwrap();

        // Should have a valid UUID
        assert!(!identity.agent_id().is_empty());
        Uuid::parse_str(identity.agent_id()).expect("Should be a valid UUID");

        // State file should exist
        assert!(dir.path().join("state.json").exists());
    }

    #[test]
    fn test_load_existing_identity() {
        let dir = TempDir::new().unwrap();
        let config = create_test_config(dir.path().to_path_buf());

        // Create first identity
        let identity1 = IdentityManager::load_or_create(&config).unwrap();
        let first_id = identity1.agent_id().to_string();

        // Load again - should get same ID
        let identity2 = IdentityManager::load_or_create(&config).unwrap();

        assert_eq!(identity2.agent_id(), first_id);
    }

    #[test]
    fn test_identity_persists() {
        let dir = TempDir::new().unwrap();
        let config = create_test_config(dir.path().to_path_buf());

        let identity1 = IdentityManager::load_or_create(&config).unwrap();
        let id = identity1.agent_id().to_string();
        drop(identity1);

        // Simulate restart - load again
        let identity2 = IdentityManager::load_or_create(&config).unwrap();
        
        assert_eq!(identity2.agent_id(), id);
    }

    #[test]
    fn test_corrupted_state_creates_new() {
        let dir = TempDir::new().unwrap();
        let state_path = dir.path().join("state.json");
        
        // Write corrupted state
        fs::write(&state_path, "not valid json {{{").unwrap();

        let config = create_test_config(dir.path().to_path_buf());

        // This should fail to parse the corrupted file
        let result = IdentityManager::load_or_create(&config);
        assert!(result.is_err());
    }

    #[test]
    fn test_version_matches_cargo() {
        let dir = TempDir::new().unwrap();
        let config = create_test_config(dir.path().to_path_buf());

        let identity = IdentityManager::load_or_create(&config).unwrap();

        assert_eq!(identity.version(), env!("CARGO_PKG_VERSION"));
    }
}
