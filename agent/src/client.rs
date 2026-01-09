//! HTTP client for Sennet Control Plane
//!
//! Communicates with the backend using ConnectRPC protocol over HTTP.

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

use crate::config::Config;

/// Metrics summary sent with heartbeat
#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct MetricsSummary {
    pub rx_packets: u64,
    pub rx_bytes: u64,
    pub tx_packets: u64,
    pub tx_bytes: u64,
    pub drop_count: u64,
    pub uptime_seconds: u64,
}

/// Heartbeat request payload
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct HeartbeatRequest {
    pub agent_id: String,
    pub current_version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metrics: Option<MetricsSummary>,
}

/// Command from server
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum Command {
    CommandUnspecified,
    CommandNoop,
    CommandUpgrade,
    CommandReconfigure,
}

impl Default for Command {
    fn default() -> Self {
        Command::CommandNoop
    }
}

/// Heartbeat response from server
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HeartbeatResponse {
    #[serde(default)]
    pub command: Command,
    #[serde(default)]
    pub latest_version: String,
    /// Config hash for change detection (Phase 10: model updates)
    #[serde(default)]
    #[allow(dead_code)]
    pub config_hash: String,
}

/// Client for the Sentinel service
pub struct SentinelClient {
    base_url: String,
    api_key: String,
}

impl SentinelClient {
    /// Create a new client
    pub fn new(config: &Config) -> Result<Self> {
        Ok(Self {
            base_url: config.server_url.trim_end_matches('/').to_string(),
            api_key: config.api_key.clone(),
        })
    }

    /// Send a heartbeat to the control plane
    pub fn heartbeat(&self, request: &HeartbeatRequest) -> Result<HeartbeatResponse> {
        let url = format!("{}/sentinel.v1.SentinelService/Heartbeat", self.base_url);

        let response = ureq::post(&url)
            .set("Authorization", &format!("Bearer {}", self.api_key))
            .set("Content-Type", "application/json")
            .send_json(request)
            .context("Failed to send heartbeat request")?;

        let resp: HeartbeatResponse = response
            .into_json()
            .context("Failed to parse heartbeat response")?;

        Ok(resp)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_heartbeat_request_serialization() {
        let request = HeartbeatRequest {
            agent_id: "test-uuid".to_string(),
            current_version: "1.0.0".to_string(),
            metrics: Some(MetricsSummary {
                rx_packets: 100,
                rx_bytes: 1000,
                tx_packets: 50,
                tx_bytes: 500,
                drop_count: 0,
                uptime_seconds: 3600,
            }),
        };

        let json = serde_json::to_string(&request).unwrap();
        assert!(json.contains("agentId"));
        assert!(json.contains("currentVersion"));
        assert!(json.contains("rxPackets"));
    }

    #[test]
    fn test_heartbeat_response_deserialization() {
        let json = r#"{
            "command": "COMMAND_NOOP",
            "latestVersion": "1.0.0",
            "configHash": "abc123"
        }"#;

        let response: HeartbeatResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.command, Command::CommandNoop);
        assert_eq!(response.latest_version, "1.0.0");
        assert_eq!(response.config_hash, "abc123");
    }

    #[test]
    fn test_upgrade_command_deserialization() {
        let json = r#"{
            "command": "COMMAND_UPGRADE",
            "latestVersion": "2.0.0",
            "configHash": "def456"
        }"#;

        let response: HeartbeatResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.command, Command::CommandUpgrade);
        assert_eq!(response.latest_version, "2.0.0");
    }

    #[test]
    fn test_empty_response() {
        let json = r#"{}"#;

        let response: HeartbeatResponse = serde_json::from_str(json).unwrap();
        assert_eq!(response.command, Command::CommandNoop);
        assert_eq!(response.latest_version, "");
    }
}
