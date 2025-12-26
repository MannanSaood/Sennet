fn main() {
    println!("cargo:rerun-if-changed=build.rs");
    
    // If embed_bpf feature is enabled
    if std::env::var("CARGO_FEATURE_EMBED_BPF").is_ok() {
        // Read path from env var (set in CI) or fallback to local relative path
        // Use CARGO_MANIFEST_DIR to be robust against CWD differences in cross
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR").unwrap();
        let source_path = std::env::var("SENNET_EBPF_BINARY")
            .unwrap_or_else(|_| std::path::Path::new(&manifest_dir).join("src").join("sennet_ebpf.bin").to_str().unwrap().to_string());

        let out_dir = std::env::var("OUT_DIR").unwrap();
        let dest_path = std::path::Path::new(&out_dir).join("sennet_ebpf.bin");
        
        // Only if file exists (don't fail build if missing, just warn - ebpf.rs handles logic)
        if std::path::Path::new(&source_path).exists() {
            std::fs::copy(&source_path, &dest_path).expect("Failed to copy eBPF binary");
            println!("cargo:warning=Embedded eBPF binary from {}", source_path);
        } else {
             // Debugging: List files in manifest dir to see what is mounted
             eprintln!("DEBUG: Current Dir: {:?}", std::env::current_dir());
             eprintln!("DEBUG: Manifest Dir: {}", manifest_dir);
             if let Ok(entries) = std::fs::read_dir(&manifest_dir) {
                 for entry in entries.flatten() {
                     eprintln!("DEBUG: Found: {:?}", entry.path());
                     if entry.path().is_dir() && entry.file_name() == "src" {
                         // List src contents too
                         if let Ok(src_entries) = std::fs::read_dir(entry.path()) {
                             for src_entry in src_entries.flatten() {
                                 eprintln!("DEBUG:   src/{:?}", src_entry.file_name());
                             }
                         }
                     }
                 }
             }
             panic!("eBPF binary NOT FOUND at {}. Cannot build with feature embed_bpf.", source_path);
        }
    }
}
