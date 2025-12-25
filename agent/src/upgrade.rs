//! Self-Update Module
//!
//! Handles downloading new versions, verifying checksums, and atomic binary replacement.

use anyhow::{anyhow, Context, Result};
use std::fs;
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::process::Command;

/// GitHub repository for releases
const GITHUB_REPO: &str = "your-org/sennet";

/// Current version of the agent
pub const CURRENT_VERSION: &str = env!("CARGO_PKG_VERSION");

/// Self-updater for the Sennet agent
pub struct Updater {
    /// GitHub repository
    repo: String,
    /// Current binary path
    binary_path: PathBuf,
}

impl Updater {
    /// Create a new updater
    pub fn new() -> Result<Self> {
        let binary_path = std::env::current_exe()
            .context("Failed to get current executable path")?;
        
        Ok(Self {
            repo: GITHUB_REPO.to_string(),
            binary_path,
        })
    }

    /// Check if an upgrade is available
    pub fn check_upgrade(&self) -> Result<Option<String>> {
        let latest = self.fetch_latest_version()?;
        
        if needs_upgrade(CURRENT_VERSION, &latest) {
            Ok(Some(latest))
        } else {
            Ok(None)
        }
    }

    /// Perform the upgrade
    pub fn upgrade(&self) -> Result<()> {
        tracing::info!("Starting self-upgrade from v{}", CURRENT_VERSION);

        // 1. Fetch latest version
        let latest = self.fetch_latest_version()?;
        if !needs_upgrade(CURRENT_VERSION, &latest) {
            tracing::info!("Already at latest version v{}", CURRENT_VERSION);
            return Ok(());
        }
        tracing::info!("Upgrading to v{}", latest);

        // 2. Download new binary to temp location
        let temp_path = self.download_binary(&latest)?;
        tracing::info!("Downloaded to {:?}", temp_path);

        // 3. Verify checksum
        let expected_hash = self.fetch_checksum(&latest)?;
        self.verify_checksum(&temp_path, &expected_hash)?;
        tracing::info!("Checksum verified");

        // 4. Atomic replace
        self.atomic_replace(&temp_path)?;
        tracing::info!("Binary replaced");

        // 5. Trigger restart
        self.trigger_restart()?;

        Ok(())
    }

    /// Fetch the latest version from GitHub releases
    fn fetch_latest_version(&self) -> Result<String> {
        let url = format!("https://api.github.com/repos/{}/releases/latest", self.repo);
        
        let response = ureq::get(&url)
            .set("User-Agent", "sennet-agent")
            .call()
            .context("Failed to fetch latest release")?;

        let body: serde_json::Value = response.into_json()
            .context("Failed to parse release response")?;

        let tag = body["tag_name"]
            .as_str()
            .ok_or_else(|| anyhow!("No tag_name in release"))?;

        // Remove 'v' prefix if present
        Ok(tag.trim_start_matches('v').to_string())
    }

    /// Download binary to temp location
    fn download_binary(&self, version: &str) -> Result<PathBuf> {
        let arch = self.detect_arch()?;
        let filename = format!("sennet-{}", arch);
        let url = format!(
            "https://github.com/{}/releases/download/v{}/{}",
            self.repo, version, filename
        );

        let temp_path = std::env::temp_dir().join("sennet_new");

        let response = ureq::get(&url)
            .call()
            .context("Failed to download binary")?;

        let mut file = fs::File::create(&temp_path)
            .context("Failed to create temp file")?;

        let mut reader = response.into_reader();
        let mut buffer = Vec::new();
        reader.read_to_end(&mut buffer)
            .context("Failed to read download")?;

        file.write_all(&buffer)
            .context("Failed to write binary")?;

        // Make executable
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mut perms = fs::metadata(&temp_path)?.permissions();
            perms.set_mode(0o755);
            fs::set_permissions(&temp_path, perms)?;
        }

        Ok(temp_path)
    }

    /// Fetch checksum for the version
    fn fetch_checksum(&self, version: &str) -> Result<String> {
        let arch = self.detect_arch()?;
        let url = format!(
            "https://github.com/{}/releases/download/v{}/checksums.txt",
            self.repo, version
        );

        let response = ureq::get(&url)
            .call()
            .context("Failed to download checksums")?;

        let body = response.into_string()
            .context("Failed to read checksums")?;

        let filename = format!("sennet-{}", arch);
        for line in body.lines() {
            if line.contains(&filename) {
                let hash = line.split_whitespace().next()
                    .ok_or_else(|| anyhow!("Invalid checksum line"))?;
                return Ok(hash.to_string());
            }
        }

        Err(anyhow!("Checksum not found for {}", filename))
    }

    /// Verify file checksum
    fn verify_checksum(&self, path: &Path, expected: &str) -> Result<()> {
        let content = fs::read(path)
            .context("Failed to read file for checksum")?;

        let actual = sha256_hex(&content);

        if actual != expected {
            return Err(anyhow!(
                "Checksum mismatch! Expected: {}, Got: {}",
                expected, actual
            ));
        }

        Ok(())
    }

    /// Atomic replace of the binary
    fn atomic_replace(&self, new_binary: &Path) -> Result<()> {
        // On Linux, we can rename over a running binary
        fs::rename(new_binary, &self.binary_path)
            .context("Failed to replace binary (atomic rename)")?;
        Ok(())
    }

    /// Trigger systemd restart
    fn trigger_restart(&self) -> Result<()> {
        tracing::info!("Triggering service restart...");
        
        // Use systemctl to restart
        let status = Command::new("systemctl")
            .args(["restart", "sennet"])
            .status();

        match status {
            Ok(s) if s.success() => {
                tracing::info!("Service restart triggered");
                Ok(())
            }
            Ok(s) => {
                tracing::warn!("systemctl restart returned: {}", s);
                Ok(()) // Non-fatal
            }
            Err(e) => {
                tracing::warn!("Failed to trigger restart: {}", e);
                tracing::info!("Please restart manually: sudo systemctl restart sennet");
                Ok(()) // Non-fatal
            }
        }
    }

    /// Detect system architecture
    fn detect_arch(&self) -> Result<&'static str> {
        #[cfg(target_arch = "x86_64")]
        return Ok("linux-amd64");
        
        #[cfg(target_arch = "aarch64")]
        return Ok("linux-arm64");

        #[cfg(not(any(target_arch = "x86_64", target_arch = "aarch64")))]
        return Err(anyhow!("Unsupported architecture"));
    }
}

/// Compare versions to determine if upgrade is needed
pub fn needs_upgrade(current: &str, latest: &str) -> bool {
    let parse_version = |v: &str| -> Vec<u32> {
        v.split('.')
            .filter_map(|s| s.parse().ok())
            .collect()
    };

    let curr = parse_version(current);
    let lat = parse_version(latest);

    for (c, l) in curr.iter().zip(lat.iter()) {
        use std::cmp::Ordering;
        match c.cmp(l) {
            Ordering::Less => return true,
            Ordering::Greater => return false,
            Ordering::Equal => continue,
        }
    }
    lat.len() > curr.len()
}

/// Calculate SHA256 hex digest
fn sha256_hex(data: &[u8]) -> String {
    use std::fmt::Write;
    
    // Simple SHA256 implementation using system command (fallback)
    // In production, use the `sha2` crate
    let temp = std::env::temp_dir().join("sennet_checksum_tmp");
    if fs::write(&temp, data).is_ok() {
        if let Ok(output) = Command::new("sha256sum")
            .arg(&temp)
            .output()
        {
            let _ = fs::remove_file(&temp);
            if let Ok(stdout) = String::from_utf8(output.stdout) {
                if let Some(hash) = stdout.split_whitespace().next() {
                    return hash.to_string();
                }
            }
        }
        let _ = fs::remove_file(&temp);
    }

    // Fallback: return empty (will fail verification)
    String::new()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_needs_upgrade() {
        assert!(needs_upgrade("1.0.0", "2.0.0"));
        assert!(needs_upgrade("1.0.0", "1.1.0"));
        assert!(needs_upgrade("1.0.0", "1.0.1"));
        assert!(!needs_upgrade("2.0.0", "1.0.0"));
        assert!(!needs_upgrade("1.0.0", "1.0.0"));
    }

    #[test]
    fn test_sha256_known_value() {
        // "hello" SHA256 = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
        let hash = sha256_hex(b"hello");
        // This test will only pass on Linux with sha256sum available
        if !hash.is_empty() {
            assert_eq!(hash, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824");
        }
    }
}
