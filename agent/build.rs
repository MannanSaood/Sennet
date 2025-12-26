fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    
    // Dump all environment variables for debugging
    println!("cargo:warning==== DEBUG: ENVIRONMENT VARIABLES ===");
    for (key, value) in std::env::vars() {
        if key.starts_with("CARGO") || key.starts_with("SENNET") || key.contains("DIR") {
            println!("cargo:warning={}={}", key, value);
        }
    }
    println!("cargo:warning===================================");

    let current_dir = std::env::current_dir().unwrap();
    println!("cargo:warning=CWD: {:?}", current_dir);
    
    // Recursive ls helper
    fn list_files(path: &std::path::Path, depth: usize) {
        if depth > 3 { return; } // Limit depth
        if let Ok(entries) = std::fs::read_dir(path) {
            for entry in entries.flatten() {
                let p = entry.path();
                println!("cargo:warning=LS: {:?}", p);
                if p.is_dir() {
                    list_files(&p, depth + 1);
                }
            }
        } else {
             println!("cargo:warning=LS: Cannot read {:?}", path);
        }
    }

    println!("cargo:warning==== DEBUG: FILESYSTEM (CWD) ===");
    list_files(&current_dir, 0);
    println!("cargo:warning===============================");

    // Also try to list /project if it exists (common cross mount point)
    let project_path = std::path::Path::new("/project");
    if project_path.exists() {
        println!("cargo:warning==== DEBUG: FILESYSTEM (/project) ===");
        list_files(project_path, 0);
        println!("cargo:warning===================================");
    }

    // If embed_bpf feature is enabled
    if std::env::var("CARGO_FEATURE_EMBED_BPF").is_ok() {
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
        let manifest_path = std::path::Path::new(&manifest_dir);

        // Try multiple candidate paths
        let candidates = vec![
            std::env::var("SENNET_EBPF_BINARY").unwrap_or_default(), // Env var
            manifest_path.join("src").join("sennet_ebpf.bin").to_string_lossy().to_string(), // src/sennet_ebpf.bin
            manifest_path.join("sennet_ebpf.bin").to_string_lossy().to_string(), // ./sennet_ebpf.bin
            "sennet_ebpf.bin".to_string(), // Relative CWD
            "src/sennet_ebpf.bin".to_string(), // Relative src
        ];

        let out_dir = std::env::var("OUT_DIR").unwrap();
        let dest_path = std::path::Path::new(&out_dir).join("sennet_ebpf.bin");
        
        let mut found = false;
        
        for source_path in candidates {
            if source_path.is_empty() { continue; }
            let p = std::path::Path::new(&source_path);
            if p.exists() {
                println!("cargo:warning=FOUND binary at {:?}", p);
                std::fs::copy(&p, &dest_path).expect("Failed to copy eBPF binary");
                println!("cargo:warning=Copied to {:?}", dest_path);
                found = true;
                break;
            } else {
                println!("cargo:warning=Candidate NOT FOUND: {:?}", p);
            }
        }

        if !found {
             panic!("eBPF binary NOT FOUND. Check the logs above for 'LS' output.");
        }
    }
}
