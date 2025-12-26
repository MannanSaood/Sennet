fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    
    // If embed_bpf feature is enabled, copy the eBPF binary to OUT_DIR
    if std::env::var("CARGO_FEATURE_EMBED_BPF").is_ok() {
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
        let manifest_path = std::path::Path::new(&manifest_dir);
        let out_dir = std::env::var("OUT_DIR").unwrap();
        let dest_path = std::path::Path::new(&out_dir).join("sennet_ebpf.bin");

        // Try multiple candidate paths in order of preference
        let candidates = vec![
            std::env::var("SENNET_EBPF_BINARY").unwrap_or_default(),
            manifest_path.join("sennet_ebpf.bin").to_string_lossy().to_string(),
            manifest_path.join("src").join("sennet_ebpf.bin").to_string_lossy().to_string(),
            "sennet_ebpf.bin".to_string(),
            "src/sennet_ebpf.bin".to_string(),
        ];

        let mut found = false;
        for source_path in &candidates {
            if source_path.is_empty() { continue; }
            let p = std::path::Path::new(source_path);
            if p.exists() {
                println!("cargo:warning=Found eBPF binary at {:?}", p);
                std::fs::copy(p, &dest_path).expect("Failed to copy eBPF binary");
                found = true;
                break;
            }
        }

        if !found {
            eprintln!("ERROR: eBPF binary not found in any candidate path:");
            for c in &candidates {
                if !c.is_empty() {
                    eprintln!("  - {}", c);
                }
            }
            eprintln!("CWD: {:?}", std::env::current_dir().unwrap());
            eprintln!("CARGO_MANIFEST_DIR: {}", manifest_dir);
            panic!("eBPF binary required for embed_bpf feature but not found!");
        }
    }
}

