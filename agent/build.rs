fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    
    // If embed_bpf feature is enabled
    if std::env::var("CARGO_FEATURE_EMBED_BPF").is_ok() {
        // Read path from env var (set in CI) or fallback to local relative path
        let source_path = std::env::var("SENNET_EBPF_BINARY")
            .unwrap_or_else(|_| "src/sennet_ebpf.bin".to_string());
            
        let out_dir = std::env::var("OUT_DIR").unwrap();
        let dest_path = std::path::Path::new(&out_dir).join("sennet_ebpf.bin");
        
        // Only if file exists (don't fail build if missing, just warn - ebpf.rs handles logic)
        if std::path::Path::new(&source_path).exists() {
            std::fs::copy(&source_path, &dest_path).expect("Failed to copy eBPF binary");
            println!("cargo:warning=Embedded eBPF binary from {}", source_path);
        } else {
             println!("cargo:warning=eBPF binary NOT FOUND at {}. Build might fail at runtime.", source_path);
             // Create dummy file to prevent include_bytes fail if we were using it, 
             // but we will switch to reading from OUT_DIR at compile time?
             // Actually, include_bytes! requires file at compile time relative to source.
             
             // Better: Copy to OUT_DIR and use include_bytes!(concat!(env!("OUT_DIR"), "/sennet_ebpf.bin"))
        }
    }
}
