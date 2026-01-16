//! Kubernetes Context & NetworkPolicy Correlation (Phase 7)
//!
//! This module provides:
//! - Container ID to Pod mapping (7.1)
//! - NetworkPolicy fetching and indexing (7.2)
//! - CNI detection (7.3)
//! - Connectivity diagnosis (7.4)

use anyhow::{Context, Result};
use std::collections::{BTreeMap, HashMap};
use std::path::Path;
use std::sync::Arc;
use tokio::sync::RwLock;
use tracing::{debug, info, warn};

#[cfg(target_os = "linux")]
use std::fs;

// =============================================================================
// Types and Data Structures
// =============================================================================

/// Pod information enriched from Kubernetes API
#[derive(Debug, Clone)]
pub struct PodInfo {
    pub name: String,
    pub namespace: String,
    pub labels: HashMap<String, String>,
    pub node_name: String,
    pub ip: Option<String>,
    pub container_ids: Vec<String>,
}

/// Container to Pod mapping entry
#[derive(Debug, Clone)]
#[allow(dead_code)] // Reserved for future flow enrichment
pub struct ContainerMapping {
    pub container_id: String,
    pub pod_info: PodInfo,
}

/// NetworkPolicy rule summary
#[derive(Debug, Clone)]
pub struct NetworkPolicyInfo {
    pub name: String,
    pub namespace: String,
    pub pod_selector: HashMap<String, String>,
    pub policy_types: Vec<String>, // "Ingress", "Egress"
    pub ingress_rules: Vec<PolicyRule>,
    pub egress_rules: Vec<PolicyRule>,
}

/// A single policy rule (simplified)
#[derive(Debug, Clone)]
#[allow(dead_code)] // Fields used for policy analysis
pub struct PolicyRule {
    pub from_pod_selector: Option<HashMap<String, String>>,
    pub from_namespace_selector: Option<HashMap<String, String>>,
    pub ports: Vec<PolicyPort>,
}

/// Port specification in a policy
#[derive(Debug, Clone)]
#[allow(dead_code)] // Fields used for policy analysis
pub struct PolicyPort {
    pub protocol: String,
    pub port: Option<u16>,
}

/// CNI (Container Network Interface) type detected
#[derive(Debug, Clone, PartialEq)]
#[allow(dead_code)] // Variants used in CNI detection logic
pub enum CniType {
    Calico,
    Cilium,
    Flannel,
    WeaveNet,
    Kindnet,
    AwsVpcCni,
    AzureCni,
    Generic,
    Unknown,
}

impl std::fmt::Display for CniType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CniType::Calico => write!(f, "Calico"),
            CniType::Cilium => write!(f, "Cilium"),
            CniType::Flannel => write!(f, "Flannel"),
            CniType::WeaveNet => write!(f, "Weave Net"),
            CniType::Kindnet => write!(f, "kindnet"),
            CniType::AwsVpcCni => write!(f, "AWS VPC CNI"),
            CniType::AzureCni => write!(f, "Azure CNI"),
            CniType::Generic => write!(f, "Generic"),
            CniType::Unknown => write!(f, "Unknown"),
        }
    }
}

/// Diagnosis result for connectivity check
#[derive(Debug)]
pub struct DiagnosisResult {
    pub source_pod: Option<PodInfo>,
    pub target_pod: Option<PodInfo>,
    pub blocking_policies: Vec<NetworkPolicyInfo>,
    pub recommendations: Vec<String>,
    pub connectivity_status: ConnectivityStatus,
}

#[derive(Debug, Clone, PartialEq)]
pub enum ConnectivityStatus {
    Allowed,
    Blocked,
    Unknown,
}

// =============================================================================
// Kubernetes Manager (7.1 & 7.2)
// =============================================================================

/// Kubernetes context manager for pod and policy enrichment
pub struct K8sManager {
    /// Container ID -> Pod mapping cache
    container_cache: Arc<RwLock<HashMap<String, PodInfo>>>,
    /// NetworkPolicy index by namespace
    policy_index: Arc<RwLock<HashMap<String, Vec<NetworkPolicyInfo>>>>,
    /// Detected CNI type
    cni_type: CniType,
    /// Whether we're running inside a Kubernetes cluster
    in_cluster: bool,
}

impl K8sManager {
    /// Create a new K8s manager
    /// 
    /// Attempts to detect if running in a Kubernetes cluster and what CNI is in use.
    /// Supports both in-cluster and out-of-cluster (kubeconfig) modes.
    pub async fn new() -> Result<Self> {
        let in_cluster = Self::detect_in_cluster();
        let has_kubeconfig = Self::detect_kubeconfig();
        let cni_type = Self::detect_cni();
        
        info!(
            "K8s Manager initialized: in_cluster={}, kubeconfig={}, cni={}",
            in_cluster, has_kubeconfig, cni_type
        );
        
        Ok(Self {
            container_cache: Arc::new(RwLock::new(HashMap::new())),
            policy_index: Arc::new(RwLock::new(HashMap::new())),
            cni_type,
            in_cluster: in_cluster || has_kubeconfig, // Consider "in cluster" if we have any K8s access
        })
    }
    
    /// Check if we're running inside a Kubernetes cluster
    fn detect_in_cluster() -> bool {
        // Check for service account token (mounted in pods)
        Path::new("/var/run/secrets/kubernetes.io/serviceaccount/token").exists()
    }
    
    /// Check if we have kubeconfig access (out-of-cluster mode)
    fn detect_kubeconfig() -> bool {
        // Check KUBECONFIG env var
        if std::env::var("KUBECONFIG").is_ok() {
            return true;
        }
        
        // Check default kubeconfig location
        if let Some(home) = std::env::var("HOME").ok() {
            let default_kubeconfig = Path::new(&home).join(".kube/config");
            if default_kubeconfig.exists() {
                return true;
            }
        }
        
        false
    }
    
    /// Detect the CNI in use (Phase 7.3)
    #[cfg(target_os = "linux")]
    fn detect_cni() -> CniType {
        let cni_config_dir = Path::new("/etc/cni/net.d");
        
        if !cni_config_dir.exists() {
            return CniType::Unknown;
        }
        
        // Read CNI config files and look for hints
        if let Ok(entries) = fs::read_dir(cni_config_dir) {
            for entry in entries.flatten() {
                let path = entry.path();
                if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
                    let name_lower = name.to_lowercase();
                    
                    // Check filename for CNI hints
                    if name_lower.contains("calico") {
                        return CniType::Calico;
                    }
                    if name_lower.contains("cilium") {
                        return CniType::Cilium;
                    }
                    if name_lower.contains("flannel") {
                        return CniType::Flannel;
                    }
                    if name_lower.contains("weave") {
                        return CniType::WeaveNet;
                    }
                    if name_lower.contains("kindnet") {
                        return CniType::Kindnet;
                    }
                    if name_lower.contains("aws") {
                        return CniType::AwsVpcCni;
                    }
                    if name_lower.contains("azure") {
                        return CniType::AzureCni;
                    }
                    
                    // Also check file contents
                    if let Ok(contents) = fs::read_to_string(&path) {
                        let contents_lower = contents.to_lowercase();
                        if contents_lower.contains("calico") {
                            return CniType::Calico;
                        }
                        if contents_lower.contains("cilium") {
                            return CniType::Cilium;
                        }
                        if contents_lower.contains("flannel") {
                            return CniType::Flannel;
                        }
                    }
                }
            }
        }
        
        // Check for CNI-specific pods/namespaces
        if Path::new("/sys/fs/bpf/cilium").exists() {
            return CniType::Cilium;
        }
        
        CniType::Generic
    }
    
    #[cfg(not(target_os = "linux"))]
    fn detect_cni() -> CniType {
        CniType::Unknown
    }
    
    /// Get the detected CNI type
    #[allow(dead_code)] // Reserved for future enrichment
    pub fn cni_type(&self) -> &CniType {
        &self.cni_type
    }
    
    /// Check if running in Kubernetes
    pub fn is_in_cluster(&self) -> bool {
        self.in_cluster
    }
    
    /// Look up pod info by container ID
    #[allow(dead_code)] // Reserved for flow enrichment
    pub async fn get_pod_by_container(&self, container_id: &str) -> Option<PodInfo> {
        let cache = self.container_cache.read().await;
        cache.get(container_id).cloned()
    }
    
    /// Look up pod info by IP address
    #[allow(dead_code)] // Reserved for flow enrichment
    pub async fn get_pod_by_ip(&self, ip: &str) -> Option<PodInfo> {
        let cache = self.container_cache.read().await;
        cache.values().find(|p| p.ip.as_deref() == Some(ip)).cloned()
    }
    
    /// Get all NetworkPolicies affecting a pod
    pub async fn get_policies_for_pod(&self, namespace: &str, labels: &HashMap<String, String>) -> Vec<NetworkPolicyInfo> {
        let index = self.policy_index.read().await;
        let mut matching = Vec::new();
        
        if let Some(policies) = index.get(namespace) {
            for policy in policies {
                if Self::labels_match(&policy.pod_selector, labels) {
                    matching.push(policy.clone());
                }
            }
        }
        
        matching
    }
    
    /// Check if a label selector matches a set of labels
    fn labels_match(selector: &HashMap<String, String>, labels: &HashMap<String, String>) -> bool {
        // Empty selector matches everything
        if selector.is_empty() {
            return true;
        }
        
        // All selector labels must be present and match
        for (key, value) in selector {
            if labels.get(key) != Some(value) {
                return false;
            }
        }
        true
    }
    
    /// Start the background sync loop for pod and policy caching
    pub async fn start_sync(&self) -> Result<()> {
        if !self.in_cluster {
            info!("Not in Kubernetes cluster, skipping K8s sync");
            return Ok(());
        }
        
        let container_cache = Arc::clone(&self.container_cache);
        let policy_index = Arc::clone(&self.policy_index);
        
        // Spawn background task for syncing
        tokio::spawn(async move {
            if let Err(e) = Self::sync_loop(container_cache, policy_index).await {
                warn!("K8s sync loop error: {}", e);
            }
        });
        
        Ok(())
    }
    
    /// Background sync loop
    async fn sync_loop(
        container_cache: Arc<RwLock<HashMap<String, PodInfo>>>,
        policy_index: Arc<RwLock<HashMap<String, Vec<NetworkPolicyInfo>>>>,
    ) -> Result<()> {
        use futures::StreamExt;
        use k8s_openapi::api::core::v1::Pod;
        use k8s_openapi::api::networking::v1::NetworkPolicy;
        use kube::{Api, Client, runtime::watcher, runtime::watcher::Event};
        
        let client = Client::try_default().await
            .context("Failed to create Kubernetes client")?;
        
        info!("Connected to Kubernetes API, starting watchers");
        
        // Watch pods across all namespaces
        let pods: Api<Pod> = Api::all(client.clone());
        let policies: Api<NetworkPolicy> = Api::all(client.clone());
        
        // Spawn pod watcher
        let cache_clone = Arc::clone(&container_cache);
        let pod_watcher = tokio::spawn(async move {
            let mut stream = watcher(pods, watcher::Config::default()).boxed();
            
            while let Some(event) = stream.next().await {
                match event {
                    Ok(Event::Applied(pod)) => {
                        if let Some(info) = Self::pod_to_info(&pod) {
                            let mut cache = cache_clone.write().await;
                            for cid in &info.container_ids {
                                cache.insert(cid.clone(), info.clone());
                            }
                            debug!("Cached pod: {}/{}", info.namespace, info.name);
                        }
                    }
                    Ok(Event::Deleted(pod)) => {
                        if let Some(info) = Self::pod_to_info(&pod) {
                            let mut cache = cache_clone.write().await;
                            for cid in &info.container_ids {
                                cache.remove(cid);
                            }
                            debug!("Removed pod: {}/{}", info.namespace, info.name);
                        }
                    }
                    Ok(Event::Restarted(pods)) => {
                        // Initial list on (re)start - cache all pods
                        let mut cache = cache_clone.write().await;
                        for pod in pods {
                            if let Some(info) = Self::pod_to_info(&pod) {
                                for cid in &info.container_ids {
                                    cache.insert(cid.clone(), info.clone());
                                }
                            }
                        }
                        debug!("Pod cache initialized/restarted");
                    }
                    Err(e) => {
                        warn!("Pod watcher error: {}", e);
                    }
                }
            }
        });
        
        // Spawn policy watcher
        let index_clone = Arc::clone(&policy_index);
        let policy_watcher = tokio::spawn(async move {
            let mut stream = watcher(policies, watcher::Config::default()).boxed();
            
            while let Some(event) = stream.next().await {
                match event {
                    Ok(Event::Applied(policy)) => {
                        if let Some(info) = Self::policy_to_info(&policy) {
                            let mut index = index_clone.write().await;
                            let ns_policies = index.entry(info.namespace.clone()).or_default();
                            
                            // Remove old version if exists
                            ns_policies.retain(|p| p.name != info.name);
                            ns_policies.push(info.clone());
                            
                            debug!("Cached NetworkPolicy: {}/{}", info.namespace, info.name);
                        }
                    }
                    Ok(Event::Deleted(policy)) => {
                        if let Some(info) = Self::policy_to_info(&policy) {
                            let mut index = index_clone.write().await;
                            if let Some(ns_policies) = index.get_mut(&info.namespace) {
                                ns_policies.retain(|p| p.name != info.name);
                            }
                            debug!("Removed NetworkPolicy: {}/{}", info.namespace, info.name);
                        }
                    }
                    Ok(Event::Restarted(policies)) => {
                        // Initial list on (re)start - index all policies
                        let mut index = index_clone.write().await;
                        for policy in policies {
                            if let Some(info) = Self::policy_to_info(&policy) {
                                let ns_policies = index.entry(info.namespace.clone()).or_default();
                                ns_policies.retain(|p| p.name != info.name);
                                ns_policies.push(info);
                            }
                        }
                        debug!("NetworkPolicy index initialized/restarted");
                    }
                    Err(e) => {
                        warn!("Policy watcher error: {}", e);
                    }
                }
            }
        });
        
        // Wait for both watchers
        let _ = tokio::try_join!(pod_watcher, policy_watcher);
        
        Ok(())
    }
    
    /// Convert a K8s Pod resource to our PodInfo
    fn pod_to_info(pod: &k8s_openapi::api::core::v1::Pod) -> Option<PodInfo> {
        let metadata = pod.metadata.clone();
        let spec = pod.spec.as_ref()?;
        let status = pod.status.as_ref()?;
        
        let name = metadata.name?;
        let namespace = metadata.namespace.unwrap_or_else(|| "default".to_string());
        // Convert BTreeMap to HashMap
        let labels: HashMap<String, String> = metadata.labels
            .unwrap_or_default()
            .into_iter()
            .collect();
        let node_name = spec.node_name.clone().unwrap_or_default();
        let ip = status.pod_ip.clone();
        
        // Extract container IDs
        let mut container_ids = Vec::new();
        if let Some(container_statuses) = &status.container_statuses {
            for cs in container_statuses {
                if let Some(cid) = &cs.container_id {
                    // Container ID format: "containerd://abc123..." or "docker://abc123..."
                    let id = cid.split("://").last().unwrap_or(cid);
                    container_ids.push(id.to_string());
                }
            }
        }
        
        Some(PodInfo {
            name,
            namespace,
            labels,
            node_name,
            ip,
            container_ids,
        })
    }
    
    /// Convert a K8s NetworkPolicy resource to our NetworkPolicyInfo
    fn policy_to_info(policy: &k8s_openapi::api::networking::v1::NetworkPolicy) -> Option<NetworkPolicyInfo> {
        let metadata = policy.metadata.clone();
        let spec = policy.spec.as_ref()?;
        
        let name = metadata.name?;
        let namespace = metadata.namespace.unwrap_or_else(|| "default".to_string());
        
        // Parse pod selector (convert BTreeMap to HashMap)
        let pod_selector: HashMap<String, String> = spec.pod_selector.match_labels
            .clone()
            .unwrap_or_default()
            .into_iter()
            .collect();
        
        // Parse policy types
        let policy_types = spec.policy_types.clone().unwrap_or_default();
        
        // Helper to convert BTreeMap to HashMap
        fn btree_to_hash(btree: Option<BTreeMap<String, String>>) -> Option<HashMap<String, String>> {
            btree.map(|b| b.into_iter().collect())
        }
        
        // Parse ingress rules
        let ingress_rules = spec.ingress.as_ref().map(|rules| {
            rules.iter().filter_map(|rule| {
                let ports = rule.ports.as_ref().map(|ps| {
                    ps.iter().map(|p| PolicyPort {
                        protocol: p.protocol.clone().unwrap_or_else(|| "TCP".to_string()),
                        port: p.port.as_ref().and_then(|port| {
                            match port {
                                k8s_openapi::apimachinery::pkg::util::intstr::IntOrString::Int(i) => Some(*i as u16),
                                _ => None,
                            }
                        }),
                    }).collect()
                }).unwrap_or_default();
                
                Some(PolicyRule {
                    from_pod_selector: rule.from.as_ref().and_then(|f| {
                        f.first().and_then(|peer| {
                            btree_to_hash(peer.pod_selector.as_ref().and_then(|s| s.match_labels.clone()))
                        })
                    }),
                    from_namespace_selector: rule.from.as_ref().and_then(|f| {
                        f.first().and_then(|peer| {
                            btree_to_hash(peer.namespace_selector.as_ref().and_then(|s| s.match_labels.clone()))
                        })
                    }),
                    ports,
                })
            }).collect()
        }).unwrap_or_default();
        
        // Parse egress rules
        let egress_rules = spec.egress.as_ref().map(|rules| {
            rules.iter().filter_map(|rule| {
                let ports = rule.ports.as_ref().map(|ps| {
                    ps.iter().map(|p| PolicyPort {
                        protocol: p.protocol.clone().unwrap_or_else(|| "TCP".to_string()),
                        port: p.port.as_ref().and_then(|port| {
                            match port {
                                k8s_openapi::apimachinery::pkg::util::intstr::IntOrString::Int(i) => Some(*i as u16),
                                _ => None,
                            }
                        }),
                    }).collect()
                }).unwrap_or_default();
                
                Some(PolicyRule {
                    from_pod_selector: rule.to.as_ref().and_then(|t| {
                        t.first().and_then(|peer| {
                            btree_to_hash(peer.pod_selector.as_ref().and_then(|s| s.match_labels.clone()))
                        })
                    }),
                    from_namespace_selector: rule.to.as_ref().and_then(|t| {
                        t.first().and_then(|peer| {
                            btree_to_hash(peer.namespace_selector.as_ref().and_then(|s| s.match_labels.clone()))
                        })
                    }),
                    ports,
                })
            }).collect()
        }).unwrap_or_default();
        
        Some(NetworkPolicyInfo {
            name,
            namespace,
            pod_selector,
            policy_types,
            ingress_rules,
            egress_rules,
        })
    }
}

// =============================================================================
// Container ID Lookup from cgroup (7.1)
// =============================================================================

/// Parse container ID from a process's cgroup file
#[cfg(target_os = "linux")]
#[allow(dead_code)] // Reserved for cgroup-based enrichment
pub fn container_id_from_pid(pid: u32) -> Option<String> {
    let cgroup_path = format!("/proc/{}/cgroup", pid);
    let contents = fs::read_to_string(&cgroup_path).ok()?;
    
    for line in contents.lines() {
        // Format: hierarchy-ID:controller-list:cgroup-path
        // Example: 1:name=systemd:/kubepods/burstable/pod123/abc456
        let parts: Vec<&str> = line.split(':').collect();
        if parts.len() >= 3 {
            let path = parts[2];
            
            // Docker format: /docker/<container_id>
            if path.starts_with("/docker/") {
                return path.strip_prefix("/docker/").map(|s| s.to_string());
            }
            
            // Containerd/K8s format: /kubepods/.../pod<uid>/<container_id>
            if path.contains("/kubepods/") {
                // The container ID is the last path component
                return path.rsplit('/').next()
                    .filter(|s| !s.is_empty() && !s.starts_with("pod"))
                    .map(|s| s.to_string());
            }
            
            // CRI-O format similar to containerd
            if path.contains("crio-") {
                if let Some(cid) = path.rsplit("crio-").next() {
                    let cid = cid.trim_end_matches(".scope");
                    return Some(cid.to_string());
                }
            }
        }
    }
    
    None
}

#[cfg(not(target_os = "linux"))]
#[allow(dead_code)]
pub fn container_id_from_pid(_pid: u32) -> Option<String> {
    None
}

/// Get container ID from network namespace inode
#[cfg(target_os = "linux")]
#[allow(dead_code)] // Reserved for netns-based enrichment
pub fn container_id_from_netns(netns_inode: u64) -> Option<String> {
    // Walk /proc to find processes with matching netns
    let proc_dir = Path::new("/proc");
    
    if let Ok(entries) = fs::read_dir(proc_dir) {
        for entry in entries.flatten() {
            let name = entry.file_name();
            if let Some(name_str) = name.to_str() {
                if let Ok(pid) = name_str.parse::<u32>() {
                    // Check this process's network namespace
                    let ns_path = format!("/proc/{}/ns/net", pid);
                    if let Ok(link) = fs::read_link(&ns_path) {
                        // Format: net:[inode]
                        if let Some(inode_str) = link.to_str() {
                            if let Some(inode) = inode_str
                                .strip_prefix("net:[")
                                .and_then(|s| s.strip_suffix(']'))
                                .and_then(|s| s.parse::<u64>().ok())
                            {
                                if inode == netns_inode {
                                    return container_id_from_pid(pid);
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    
    None
}

#[cfg(not(target_os = "linux"))]
#[allow(dead_code)]
pub fn container_id_from_netns(_netns_inode: u64) -> Option<String> {
    None
}

// =============================================================================
// Diagnosis Command (7.4)
// =============================================================================

impl K8sManager {
    /// Diagnose connectivity between two pods
    /// 
    /// Usage: `sennet diagnose frontend-pod backend-pod`
    /// 
    /// Works both in-cluster and out-of-cluster (with kubeconfig).
    pub async fn diagnose_connectivity(
        &self,
        source_ref: &str,
        target_ref: &str,
        namespace: Option<&str>,
    ) -> Result<DiagnosisResult> {
        use k8s_openapi::api::core::v1::Pod;
        use kube::{Api, Client};
        
        let client = Client::try_default().await?;
        let ns = namespace.unwrap_or("default");
        let pods: Api<Pod> = Api::namespaced(client.clone(), ns);
        
        // Look up source pod
        let source_pod = pods.get(source_ref).await.ok();
        let source_info = source_pod.as_ref().and_then(Self::pod_to_info);
        
        // Look up target pod
        let target_pod = pods.get(target_ref).await.ok();
        let target_info = target_pod.as_ref().and_then(Self::pod_to_info);
        
        let mut recommendations = Vec::new();
        let mut blocking_policies = Vec::new();
        let mut status = ConnectivityStatus::Unknown;
        
        // Check if pods exist
        if source_info.is_none() {
            recommendations.push(format!("Source pod '{}' not found in namespace '{}'", source_ref, ns));
        }
        if target_info.is_none() {
            recommendations.push(format!("Target pod '{}' not found in namespace '{}'", target_ref, ns));
        }
        
        // If both pods exist, check NetworkPolicies
        if let (Some(src), Some(tgt)) = (&source_info, &target_info) {
            // Get policies affecting source (egress)
            let src_policies = self.get_policies_for_pod(&src.namespace, &src.labels).await;
            
            // Get policies affecting target (ingress)
            let tgt_policies = self.get_policies_for_pod(&tgt.namespace, &tgt.labels).await;
            
            // Check for default deny
            let has_egress_policy = src_policies.iter().any(|p| p.policy_types.contains(&"Egress".to_string()));
            let has_ingress_policy = tgt_policies.iter().any(|p| p.policy_types.contains(&"Ingress".to_string()));
            
            if has_egress_policy {
                // Default deny egress - need explicit allow
                let allows_egress = src_policies.iter().any(|p| {
                    p.egress_rules.iter().any(|rule| {
                        // Check if rule allows traffic to target
                        if let Some(selector) = &rule.from_pod_selector {
                            Self::labels_match(selector, &tgt.labels)
                        } else {
                            // No selector = allow all
                            true
                        }
                    })
                });
                
                if !allows_egress && !src_policies.is_empty() {
                    blocking_policies.extend(src_policies.iter().filter(|p| 
                        p.policy_types.contains(&"Egress".to_string())
                    ).cloned());
                    recommendations.push(format!(
                        "Source pod '{}' has egress NetworkPolicy that may block traffic to '{}'",
                        src.name, tgt.name
                    ));
                }
            }
            
            if has_ingress_policy {
                let allows_ingress = tgt_policies.iter().any(|p| {
                    p.ingress_rules.iter().any(|rule| {
                        if let Some(selector) = &rule.from_pod_selector {
                            Self::labels_match(selector, &src.labels)
                        } else {
                            true
                        }
                    })
                });
                
                if !allows_ingress && !tgt_policies.is_empty() {
                    blocking_policies.extend(tgt_policies.iter().filter(|p|
                        p.policy_types.contains(&"Ingress".to_string())
                    ).cloned());
                    recommendations.push(format!(
                        "Target pod '{}' has ingress NetworkPolicy that may block traffic from '{}'",
                        tgt.name, src.name
                    ));
                }
            }
            
            // Determine status
            if blocking_policies.is_empty() {
                status = ConnectivityStatus::Allowed;
                recommendations.push("No blocking NetworkPolicies detected".to_string());
            } else {
                status = ConnectivityStatus::Blocked;
            }
            
            // Add CNI-specific recommendations
            match &self.cni_type {
                CniType::Calico => {
                    recommendations.push("Tip: Use 'calicoctl get networkpolicy -A' for Calico-specific policies".to_string());
                }
                CniType::Cilium => {
                    recommendations.push("Tip: Use 'cilium policy get' for Cilium policy status".to_string());
                }
                _ => {}
            }
        }
        
        Ok(DiagnosisResult {
            source_pod: source_info,
            target_pod: target_info,
            blocking_policies,
            recommendations,
            connectivity_status: status,
        })
    }
}

// =============================================================================
// Display Formatting
// =============================================================================

impl DiagnosisResult {
    /// Format diagnosis result for CLI output
    pub fn format_output(&self) -> String {
        use std::fmt::Write;
        let mut output = String::new();
        
        writeln!(output, "\n╔══════════════════════════════════════════════════════════════╗").unwrap();
        writeln!(output, "║                  SENNET CONNECTIVITY DIAGNOSIS                ║").unwrap();
        writeln!(output, "╚══════════════════════════════════════════════════════════════╝\n").unwrap();
        
        // Source pod
        writeln!(output, "┌─ SOURCE POD ─────────────────────────────────────────────────┐").unwrap();
        if let Some(pod) = &self.source_pod {
            writeln!(output, "│  Name:      {}", pod.name).unwrap();
            writeln!(output, "│  Namespace: {}", pod.namespace).unwrap();
            writeln!(output, "│  IP:        {}", pod.ip.as_deref().unwrap_or("N/A")).unwrap();
            writeln!(output, "│  Node:      {}", pod.node_name).unwrap();
        } else {
            writeln!(output, "│  NOT FOUND").unwrap();
        }
        writeln!(output, "└──────────────────────────────────────────────────────────────┘\n").unwrap();
        
        // Target pod
        writeln!(output, "┌─ TARGET POD ─────────────────────────────────────────────────┐").unwrap();
        if let Some(pod) = &self.target_pod {
            writeln!(output, "│  Name:      {}", pod.name).unwrap();
            writeln!(output, "│  Namespace: {}", pod.namespace).unwrap();
            writeln!(output, "│  IP:        {}", pod.ip.as_deref().unwrap_or("N/A")).unwrap();
            writeln!(output, "│  Node:      {}", pod.node_name).unwrap();
        } else {
            writeln!(output, "│  NOT FOUND").unwrap();
        }
        writeln!(output, "└──────────────────────────────────────────────────────────────┘\n").unwrap();
        
        // Status
        let status_str = match self.connectivity_status {
            ConnectivityStatus::Allowed => "✓ ALLOWED",
            ConnectivityStatus::Blocked => "✗ BLOCKED",
            ConnectivityStatus::Unknown => "? UNKNOWN",
        };
        writeln!(output, "CONNECTIVITY STATUS: {}\n", status_str).unwrap();
        
        // Blocking policies
        if !self.blocking_policies.is_empty() {
            writeln!(output, "BLOCKING NETWORK POLICIES:").unwrap();
            for policy in &self.blocking_policies {
                writeln!(output, "  • {}/{} (types: {:?})", 
                    policy.namespace, policy.name, policy.policy_types).unwrap();
            }
            writeln!(output).unwrap();
        }
        
        // Recommendations
        if !self.recommendations.is_empty() {
            writeln!(output, "RECOMMENDATIONS:").unwrap();
            for rec in &self.recommendations {
                writeln!(output, "  → {}", rec).unwrap();
            }
        }
        
        output
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_labels_match_empty_selector() {
        let selector = HashMap::new();
        let labels: HashMap<String, String> = [
            ("app".to_string(), "frontend".to_string()),
        ].into_iter().collect();
        
        assert!(K8sManager::labels_match(&selector, &labels));
    }
    
    #[test]
    fn test_labels_match_matching() {
        let selector: HashMap<String, String> = [
            ("app".to_string(), "frontend".to_string()),
        ].into_iter().collect();
        let labels: HashMap<String, String> = [
            ("app".to_string(), "frontend".to_string()),
            ("version".to_string(), "v1".to_string()),
        ].into_iter().collect();
        
        assert!(K8sManager::labels_match(&selector, &labels));
    }
    
    #[test]
    fn test_labels_match_not_matching() {
        let selector: HashMap<String, String> = [
            ("app".to_string(), "backend".to_string()),
        ].into_iter().collect();
        let labels: HashMap<String, String> = [
            ("app".to_string(), "frontend".to_string()),
        ].into_iter().collect();
        
        assert!(!K8sManager::labels_match(&selector, &labels));
    }
    
    #[test]
    fn test_cni_type_display() {
        assert_eq!(CniType::Calico.to_string(), "Calico");
        assert_eq!(CniType::Cilium.to_string(), "Cilium");
        assert_eq!(CniType::Unknown.to_string(), "Unknown");
    }
    
    #[test]
    fn test_diagnosis_result_format() {
        let result = DiagnosisResult {
            source_pod: Some(PodInfo {
                name: "frontend".to_string(),
                namespace: "default".to_string(),
                labels: HashMap::new(),
                node_name: "node-1".to_string(),
                ip: Some("10.0.0.5".to_string()),
                container_ids: vec![],
            }),
            target_pod: None,
            blocking_policies: vec![],
            recommendations: vec!["Target pod not found".to_string()],
            connectivity_status: ConnectivityStatus::Unknown,
        };
        
        let output = result.format_output();
        assert!(output.contains("frontend"));
        assert!(output.contains("UNKNOWN"));
    }
}
