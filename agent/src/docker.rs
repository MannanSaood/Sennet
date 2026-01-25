//! Docker Container Integration
//!
//! Provides container detection and monitoring for standalone Docker environments
//! (not Kubernetes). For K8s environments, use the k8s.rs module instead.

use anyhow::Result;
use std::collections::HashMap;
use std::path::Path;
use tracing::info;

/// Docker container information
#[derive(Debug, Clone)]
pub struct DockerContainer {
    pub id: String,
    pub name: String,
    pub image: String,
    pub pid: Option<u32>,
    pub ip: Option<String>,
    pub labels: HashMap<String, String>,
    pub state: ContainerState,
}

/// Container state
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ContainerState {
    Running,
    Paused,
    Stopped,
    Unknown,
}

/// Docker runtime type
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DockerRuntime {
    Docker,
    Containerd,
    Podman,
    Unknown,
}

/// Check if Docker daemon is available
pub fn is_docker_available() -> bool {
    // Check for Docker socket
    let docker_sock = Path::new("/var/run/docker.sock");
    if docker_sock.exists() {
        return true;
    }

    // Check for containerd socket
    let containerd_sock = Path::new("/run/containerd/containerd.sock");
    if containerd_sock.exists() {
        return true;
    }

    // Check for Podman socket
    let podman_sock = Path::new("/run/podman/podman.sock");
    if podman_sock.exists() {
        return true;
    }

    false
}

/// Detect which container runtime is in use
pub fn detect_runtime() -> DockerRuntime {
    if Path::new("/var/run/docker.sock").exists() {
        DockerRuntime::Docker
    } else if Path::new("/run/containerd/containerd.sock").exists() {
        DockerRuntime::Containerd
    } else if Path::new("/run/podman/podman.sock").exists() {
        DockerRuntime::Podman
    } else {
        DockerRuntime::Unknown
    }
}

/// Get container ID from a process's cgroup
/// This works for Docker, containerd, and Podman
#[cfg(target_os = "linux")]
pub fn get_container_id_from_pid(pid: u32) -> Option<String> {
    use std::fs;
    
    // Read the cgroup file for the process
    let cgroup_path = format!("/proc/{}/cgroup", pid);
    let content = fs::read_to_string(&cgroup_path).ok()?;
    
    for line in content.lines() {
        // Parse cgroup line: hierarchy-ID:controller:path
        let parts: Vec<&str> = line.split(':').collect();
        if parts.len() >= 3 {
            let path = parts[2];
            
            // Docker format: /docker/<container_id>
            if path.starts_with("/docker/") {
                return path.strip_prefix("/docker/")
                    .map(|s| s.to_string());
            }
            
            // Docker with systemd cgroup: /system.slice/docker-<id>.scope
            if let Some(id) = extract_docker_systemd_id(path) {
                return Some(id);
            }
            
            // Containerd format: /system.slice/containerd-<id>.scope
            if let Some(id) = extract_containerd_id(path) {
                return Some(id);
            }
            
            // Podman format: /user.slice/user-1000.slice/.../libpod-<id>.scope
            if let Some(id) = extract_podman_id(path) {
                return Some(id);
            }
        }
    }
    
    None
}

#[cfg(not(target_os = "linux"))]
pub fn get_container_id_from_pid(_pid: u32) -> Option<String> {
    None
}

/// Extract container ID from Docker systemd cgroup path
fn extract_docker_systemd_id(path: &str) -> Option<String> {
    // Format: /system.slice/docker-<id>.scope
    if path.contains("docker-") && path.ends_with(".scope") {
        let id = path.rsplit("docker-").next()?;
        let id = id.trim_end_matches(".scope");
        if id.len() >= 12 {
            return Some(id.to_string());
        }
    }
    None
}

/// Extract container ID from containerd cgroup path
fn extract_containerd_id(path: &str) -> Option<String> {
    // Multiple possible formats
    
    // Format: /system.slice/containerd-<id>.scope
    if path.contains("containerd-") && path.ends_with(".scope") {
        let id = path.rsplit("containerd-").next()?;
        let id = id.trim_end_matches(".scope");
        if id.len() >= 12 {
            return Some(id.to_string());
        }
    }
    
    // Format: /kubepods/burstable/pod.../cri-containerd-<id>.scope
    if path.contains("cri-containerd-") {
        let id = path.rsplit("cri-containerd-").next()?;
        let id = id.trim_end_matches(".scope");
        if id.len() >= 12 {
            return Some(id.to_string());
        }
    }
    
    None
}

/// Extract container ID from Podman cgroup path
fn extract_podman_id(path: &str) -> Option<String> {
    // Format: .../libpod-<id>.scope
    if path.contains("libpod-") && path.ends_with(".scope") {
        let id = path.rsplit("libpod-").next()?;
        let id = id.trim_end_matches(".scope");
        if id.len() >= 12 {
            return Some(id.to_string());
        }
    }
    None
}

/// Check if a process is running inside a container
#[cfg(target_os = "linux")]
pub fn is_process_containerized(pid: u32) -> bool {
    get_container_id_from_pid(pid).is_some()
}

#[cfg(not(target_os = "linux"))]
pub fn is_process_containerized(_pid: u32) -> bool {
    false
}

/// Check if the current process (the agent) is running in a container
#[cfg(target_os = "linux")]
pub fn is_agent_in_container() -> bool {
    // Check for container indicators
    
    // Check /.dockerenv file (Docker specific)
    if Path::new("/.dockerenv").exists() {
        return true;
    }
    
    // Check for Kubernetes-mounted service account
    if Path::new("/var/run/secrets/kubernetes.io").exists() {
        return true;
    }
    
    // Check cgroup for container ID
    let cgroup_path = "/proc/1/cgroup";
    if let Ok(content) = std::fs::read_to_string(cgroup_path) {
        for line in content.lines() {
            if line.contains("/docker/") || 
               line.contains("/kubepods/") ||
               line.contains("/libpod-") ||
               line.contains("containerd") {
                return true;
            }
        }
    }
    
    false
}

#[cfg(not(target_os = "linux"))]
pub fn is_agent_in_container() -> bool {
    false
}

/// Docker runtime info for diagnostics
#[derive(Debug)]
pub struct DockerRuntimeInfo {
    pub runtime: DockerRuntime,
    pub socket_available: bool,
    pub agent_containerized: bool,
}

/// Get Docker runtime diagnostic info
pub fn get_runtime_info() -> DockerRuntimeInfo {
    let runtime = detect_runtime();
    let socket_available = is_docker_available();
    
    #[cfg(target_os = "linux")]
    let agent_containerized = is_agent_in_container();
    
    #[cfg(not(target_os = "linux"))]
    let agent_containerized = false;
    
    info!("Docker runtime: {:?}, socket: {}, agent in container: {}", 
          runtime, socket_available, agent_containerized);
    
    DockerRuntimeInfo {
        runtime,
        socket_available,
        agent_containerized,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_docker_systemd_id() {
        let path = "/system.slice/docker-abc123def456.scope";
        let id = extract_docker_systemd_id(path);
        assert_eq!(id, Some("abc123def456".to_string()));
    }

    #[test]
    fn test_containerd_id() {
        let path = "/kubepods/burstable/pod123/cri-containerd-abc123def456.scope";
        let id = extract_containerd_id(path);
        assert_eq!(id, Some("abc123def456".to_string()));
    }

    #[test]
    fn test_podman_id() {
        let path = "/user.slice/user-1000.slice/user@1000.service/libpod-abc123def456.scope";
        let id = extract_podman_id(path);
        assert_eq!(id, Some("abc123def456".to_string()));
    }

    #[test]
    fn test_detect_runtime() {
        // Just ensure it doesn't panic
        let runtime = detect_runtime();
        println!("Runtime: {:?}", runtime);
    }
}
