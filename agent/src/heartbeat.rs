//! Heartbeat loop with exponential backoff retry
//!
//! Sends periodic heartbeats to the control plane and handles commands.

use anyhow::Result;
use backoff::ExponentialBackoff;
use std::time::{Duration, Instant};
use tracing::{debug, error, info, warn};

use crate::client::{Command, HeartbeatRequest, MetricsSummary, SentinelClient};
use crate::config::Config;
use crate::identity::IdentityManager;
use crate::upgrade::Updater;

// Linux-only: imports for reading eBPF metrics from pinned maps
#[cfg(target_os = "linux")]
use crate::ebpf::PacketCounters;
#[cfg(target_os = "linux")]
use aya::maps::{Map, MapData, PerCpuArray};
#[cfg(target_os = "linux")]
use std::path::Path;

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

    /// Collect current metrics from eBPF maps (Linux) or return zeros (other platforms)
    fn collect_metrics(&self) -> MetricsSummary {
        let uptime = self.start_time.elapsed().as_secs();
        
        #[cfg(target_os = "linux")]
        {
            // Try to read from pinned eBPF maps
            match Self::read_ebpf_counters() {
                Ok(counters) => {
                    return MetricsSummary {
                        rx_packets: counters.rx_packets,
                        rx_bytes: counters.rx_bytes,
                        tx_packets: counters.tx_packets,
                        tx_bytes: counters.tx_bytes,
                        drop_count: counters.drop_count,
                        uptime_seconds: uptime,
                    };
                }
                Err(e) => {
                    debug!("Could not read eBPF counters: {}", e);
                }
            }
        }
        
        // Fallback: return zeros (eBPF not available or not Linux)
        MetricsSummary {
            rx_packets: 0,
            rx_bytes: 0,
            tx_packets: 0,
            tx_bytes: 0,
            drop_count: 0,
            uptime_seconds: uptime,
        }
    }
    
    /// Read packet counters from pinned eBPF maps (Linux only)
    #[cfg(target_os = "linux")]
    fn read_ebpf_counters() -> Result<PacketCounters> {
        let pin_path = Path::new("/sys/fs/bpf/sennet/counters");
        if !pin_path.exists() {
            anyhow::bail!("Pinned map not found");
        }
        
        let map_data = MapData::from_pin(pin_path)?;
        let map = Map::PerCpuArray(map_data);
        let counters: PerCpuArray<_, PacketCounters> = map.try_into()?;
        
        let mut total = PacketCounters::default();
        
        // Read ingress counters (index 0)
        if let Ok(values) = counters.get(&0, 0) {
            for cpu_val in values.iter() {
                total.rx_packets += cpu_val.rx_packets;
                total.rx_bytes += cpu_val.rx_bytes;
                total.drop_count += cpu_val.drop_count;
            }
        }
        
        // Read egress counters (index 1)
        if let Ok(values) = counters.get(&1, 0) {
            for cpu_val in values.iter() {
                total.tx_packets += cpu_val.tx_packets;
                total.tx_bytes += cpu_val.tx_bytes;
            }
        }
        
        Ok(total)
    }

    /// Handle commands from the server
    fn handle_command(&self, command: &Command, latest_version: &str) {
        match command {
            Command::CommandNoop => {
                debug!("No action required");
            }
            Command::CommandUpgrade => {
                info!("Upgrade available: {} -> {}", self.identity.version(), latest_version);
                // Perform self-update
                match Updater::new() {
                    Ok(updater) => {
                        match updater.upgrade() {
                            Ok(()) => {
                                info!("Upgrade successful! Restarting...");
                                // Exec into new binary to restart
                                #[cfg(unix)]
                                {
                                    use std::os::unix::process::CommandExt;
                                    let exe = std::env::current_exe().unwrap();
                                    let err = std::process::Command::new(exe).exec();
                                    error!("Failed to exec after upgrade: {}", err);
                                }
                                #[cfg(not(unix))]
                                {
                                    warn!("Upgrade complete. Please restart the agent manually.");
                                }
                            }
                            Err(e) => {
                                error!("Upgrade failed: {}", e);
                            }
                        }
                    }
                    Err(e) => {
                        error!("Failed to initialize updater: {}", e);
                    }
                }
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
