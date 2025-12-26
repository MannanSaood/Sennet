fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    
    // If embed_bpf feature is enabled, copy the eBPF binary to OUT_DIR
    if std::env::var("CARGO_FEATURE_EMBED_BPF").is_ok() {
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
        let manifest_path = std::path::Path::new(&manifest_dir);
        let out_dir = std::env::var("OUT_DIR").unwrap();
        let dest_path = std::path::Path::new(&out_dir).join("sennet_ebpf.bin");
        let cwd = std::env::current_dir().unwrap();

        // Print debug info for CI troubleshooting
        println!("cargo:warning=build.rs: CWD = {:?}", cwd);
        println!("cargo:warning=build.rs: CARGO_MANIFEST_DIR = {}", manifest_dir);
        println!("cargo:warning=build.rs: OUT_DIR = {}", out_dir);
        if let Ok(ebpf_var) = std::env::var("SENNET_EBPF_BINARY") {
            println!("cargo:warning=build.rs: SENNET_EBPF_BINARY = {}", ebpf_var);
        }

        // Try multiple candidate paths in order of preference
        // Includes paths for cross Docker container (project mounted at /project)
        let candidates = vec![
            // Environment variable path (can be absolute or relative)
            std::env::var("SENNET_EBPF_BINARY").unwrap_or_default(),
            // Relative to CARGO_MANIFEST_DIR (works locally and in cross)
            manifest_path.join("sennet_ebpf.bin").to_string_lossy().to_string(),
            manifest_path.join("src").join("sennet_ebpf.bin").to_string_lossy().to_string(),
            // Relative to CWD
            "sennet_ebpf.bin".to_string(),
            "src/sennet_ebpf.bin".to_string(),
            // Cross mounts project at /project, agent dir is /project/agent
            // These paths work inside the cross Docker container
            "/project/agent/sennet_ebpf.bin".to_string(),
            "/project/agent/src/sennet_ebpf.bin".to_string(),
            // Also check parent directory in case CWD is agent/
            cwd.parent().map(|p| p.join("agent/sennet_ebpf.bin").to_string_lossy().to_string()).unwrap_or_default(),
        ];

        let mut found = false;
        for source_path in &candidates {
            if source_path.is_empty() { continue; }
            let p = std::path::Path::new(source_path);
            let exists = p.exists();
            println!("cargo:warning=build.rs: Checking {:?} -> exists={}", p, exists);
            if exists {
                println!("cargo:warning=build.rs: Found eBPF binary at {:?}", p);
                std::fs::copy(p, &dest_path).expect("Failed to copy eBPF binary");
                found = true;
                break;
            }
        }

        if !found {
            eprintln!("ERROR: eBPF binary not found in any candidate path:");
            for c in &candidates {
                if !c.is_empty() {
                    eprintln!("  - {} (exists: {})", c, std::path::Path::new(c).exists());
                }
            }
            eprintln!("CWD: {:?}", cwd);
            eprintln!("CARGO_MANIFEST_DIR: {}", manifest_dir);
            eprintln!("OUT_DIR: {}", out_dir);
            
            // List files in manifest dir for debugging
            if let Ok(entries) = std::fs::read_dir(&manifest_path) {
                eprintln!("Files in CARGO_MANIFEST_DIR:");
                for entry in entries.flatten() {
                    eprintln!("  - {:?}", entry.path());
                }
            }
            
            panic!("eBPF binary required for embed_bpf feature but not found!");
        }
    }
}

