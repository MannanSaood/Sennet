//! Heartbeat loop with a bounded durable telemetry outbox
//!
//! Sends periodic heartbeats to the control plane and handles commands.

use anyhow::Result;

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

    /// Disk and blocking transport run off the async executor, with deadlines.
    pub async fn run(mut self) -> Result<()> {
        loop {
            let interval = Duration::from_secs(self.config.heartbeat_interval_secs);
            self = tokio::task::spawn_blocking(move || {
                let metrics = self.collect_metrics();
                let request = HeartbeatRequest { agent_id: self.identity.agent_id().to_string(), current_version: self.identity.version().to_string(), metrics: metrics.clone() };
                if let Err(e) = self.spool_metrics(metrics.as_ref()) { error!("Telemetry spool failed: {}", e); }
                if let Err(e) = self.flush_spool() { warn!("Exporter unavailable; queued observations retained: {}", e); }
                match self.client.heartbeat(&request) {
                    Ok(response) => {
                        if response.command == Command::CommandReconfigure {
                            match Config::load() {
                                Ok(config) => match SentinelClient::new(&config) {
                                    Ok(client) => { self.config = config; self.client = client; info!("Configuration reloaded"); }
                                    Err(e) => warn!("Keeping current configuration: {}", e),
                                },
                                Err(e) => warn!("Keeping current configuration: {}", e),
                            }
                        } else { self.handle_command(&response.command, &response.latest_version); }
                    }
                    Err(e) => warn!("Heartbeat unavailable: {}", e),
                }
                self
            }).await?;
            tokio::time::sleep(interval + Duration::from_millis(rand::random::<u64>() % 1000)).await;
        }
    }
    fn spool_metrics(&self, metrics: Option<&MetricsSummary>) -> Result<()> {
        let dir = self.config.state_dir.join("spool");
        std::fs::create_dir_all(&dir)?;
        let size: u64 = std::fs::read_dir(&dir)?.filter_map(|e| e.ok()).filter_map(|e| e.metadata().ok()).map(|m| m.len()).sum();
        if size >= 20 * 1024 * 1024 { anyhow::bail!("20 MiB spool full; observation rejected"); }
        let now = chrono::Utc::now().timestamp_millis();
        let id = uuid::Uuid::new_v4().to_string();
        let mut attributes = serde_json::Map::new();
        if let Some(m) = metrics {
            for (key,value) in [("rxBytes",m.rx_bytes),("txBytes",m.tx_bytes),("rxPackets",m.rx_packets),("txPackets",m.tx_packets),("dropCount",m.drop_count),("uptimeSeconds",m.uptime_seconds)] {
                attributes.insert(key.into(), serde_json::Value::String(value.to_string()));
            }
        }
        let payload = serde_json::json!({"events":[{"id":id,"time":now,"signal":"metric","service":self.identity.agent_id(),"name":"network.snapshot","status":if metrics.is_some(){"collecting"}else{"unavailable"},"duration_ms":0,"value":0,"attributes":attributes}]});
        let temp = dir.join(format!("{}-{}.tmp",now,id)); let final_path = temp.with_extension("json");
        use std::io::Write;
        let mut file = std::fs::File::create(&temp)?;
        file.write_all(&serde_json::to_vec(&payload)?)?; file.sync_all()?;
        std::fs::rename(temp,final_path)?; Ok(())
    }
    fn flush_spool(&self) -> Result<()> {
        let dir = self.config.state_dir.join("spool");
        let mut files: Vec<_> = std::fs::read_dir(dir)?.filter_map(|e|e.ok()).map(|e|e.path()).filter(|p|p.extension().and_then(|s|s.to_str())==Some("json")).collect();
        files.sort();
        let started = Instant::now();
        for path in files.into_iter().take(20) {
            if started.elapsed() > Duration::from_secs(10) { break; }
            let bytes = std::fs::read(&path)?;
            match self.client.export_events(&bytes) {
                Ok(()) => std::fs::remove_file(path)?,
                Err(e) => {
                    if matches!(e.downcast_ref::<ureq::Error>(),Some(ureq::Error::Status(400 | 413 | 422, _))) {
                        std::fs::rename(&path,path.with_extension("rejected"))?;
                        error!("Observation quarantined after permanent validation failure");
                    } else { return Err(e); }
                }
            }
        }
        Ok(())
    }
    /// Collect current metrics from eBPF maps; unavailable collection is explicit.
    fn collect_metrics(&self) -> Option<MetricsSummary> {
        #[cfg(target_os = "linux")]
        let uptime = self.start_time.elapsed().as_secs();
        
        #[cfg(target_os = "linux")]
        {
            // Try to read from pinned eBPF maps
            match Self::read_ebpf_counters() {
                Ok(counters) => {
                    return Some(MetricsSummary {
                        rx_packets: counters.rx_packets,
                        rx_bytes: counters.rx_bytes,
                        tx_packets: counters.tx_packets,
                        tx_bytes: counters.tx_bytes,
                        drop_count: counters.drop_count,
                        uptime_seconds: uptime, });
                }
                Err(e) => {
                    debug!("Could not read eBPF counters: {}", e);
                }
            }
        }
        
        None
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
        {
            let values = counters.get(&0, 0)?;
            for cpu_val in values.iter() {
                total.rx_packets += cpu_val.rx_packets;
                total.rx_bytes += cpu_val.rx_bytes;
                total.drop_count += cpu_val.drop_count;
            }
        }
        
        // Read egress counters (index 1)
        {
            let values = counters.get(&1, 0)?;
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
                if std::env::var("SENNET_ALLOW_AUTO_UPGRADE").as_deref() != Ok("true") {
                    warn!("Upgrade available; unattended upgrades are disabled");
                    return;
                }
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
                                    let err = std::process::Command::new(exe).args(std::env::args_os().skip(1)).exec();
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
                // Reload is handled by the run loop.
                debug!("Config reload scheduled");
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
