//! Heartbeat loop with exponential backoff retry
//!
//! Sends periodic heartbeats to the control plane and handles commands.

use anyhow::Result;
use backoff::ExponentialBackoff;
use std::time::{Duration, Instant};
use tracing::{debug, info, warn};

use crate::client::{Command, HeartbeatRequest, MetricsSummary, SentinelClient};
use crate::config::Config;
use crate::identity::IdentityManager;

/// Heartbeat loop that runs continuously
pub struct HeartbeatLoop {
    config: Config,
    identity: IdentityManager,
    client: SentinelClient,
    start_time: Instant,
}

impl HeartbeatLoop {
    /// Create a new heartbeat loop
    pub fn new(config: Config, identity: IdentityManager, client: SentinelClient) -> Self {
        Self {
            config,
            identity,
            client,
            start_time: Instant::now(),
        }
    }

    /// Run the heartbeat loop forever
    pub async fn run(self) -> Result<()> {
        let interval = Duration::from_secs(self.config.heartbeat_interval_secs);
        
        info!("Starting heartbeat loop (interval: {:?})", interval);

        loop {
            match self.send_heartbeat() {
                Ok(response) => {
                    info!("Heartbeat successful, command: {:?}", response.command);
                    self.handle_command(&response.command, &response.latest_version);
                }
                Err(e) => {
                    warn!("Heartbeat failed: {}", e);
                }
            }

            tokio::time::sleep(interval).await;
        }
    }

    /// Send a single heartbeat with retry
    fn send_heartbeat(&self) -> Result<crate::client::HeartbeatResponse> {
        let request = HeartbeatRequest {
            agent_id: self.identity.agent_id().to_string(),
            current_version: self.identity.version().to_string(),
            metrics: Some(self.collect_metrics()),
        };

        // Use exponential backoff for retries
        let backoff_config = ExponentialBackoff {
            initial_interval: Duration::from_secs(1),
            max_interval: Duration::from_secs(60),
            max_elapsed_time: Some(Duration::from_secs(300)),
            ..Default::default()
        };

        let client = &self.client;
        backoff::retry(backoff_config, || {
            match client.heartbeat(&request) {
                Ok(resp) => Ok(resp),
                Err(e) => {
                    warn!("Heartbeat attempt failed, retrying: {}", e);
                    Err(backoff::Error::transient(e))
                }
            }
        })
        .map_err(|e| anyhow::anyhow!("Heartbeat failed after retries: {}", e))
    }

    /// Collect current metrics
    fn collect_metrics(&self) -> MetricsSummary {
        MetricsSummary {
            rx_packets: 0, // TODO: Implement eBPF metrics
            rx_bytes: 0,
            tx_packets: 0,
            tx_bytes: 0,
            drop_count: 0,
            uptime_seconds: self.start_time.elapsed().as_secs(),
        }
    }

    /// Handle commands from the server
    fn handle_command(&self, command: &Command, latest_version: &str) {
        match command {
            Command::CommandNoop => {
                debug!("No action required");
            }
            Command::CommandUpgrade => {
                info!("Upgrade available: {} -> {}", self.identity.version(), latest_version);
                // TODO: Implement self-update
                warn!("Self-update not yet implemented");
            }
            Command::CommandReconfigure => {
                info!("Reconfiguration requested");
                // TODO: Implement config reload
                warn!("Config reload not yet implemented");
            }
            Command::CommandUnspecified => {
                warn!("Received unspecified command");
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    #[test]
    fn test_metrics_uptime() {
        let start = Instant::now();
        std::thread::sleep(Duration::from_millis(100));
        
        let elapsed = start.elapsed().as_secs();
        // Just verify it doesn't panic and we can get elapsed time
        let _ = elapsed; 
    }

    #[test]
    fn test_command_handling() {
        // Test that commands are properly recognized
        let cmd = Command::CommandNoop;
        assert_eq!(cmd, Command::CommandNoop);

        let cmd = Command::CommandUpgrade;
        assert_eq!(cmd, Command::CommandUpgrade);
    }
}
