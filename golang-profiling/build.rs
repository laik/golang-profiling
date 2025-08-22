use anyhow::{Context as _, anyhow};
use aya_build::cargo_metadata;
use std::env;
use std::fs;
use std::path::Path;

fn main() -> anyhow::Result<()> {
    let cargo_metadata::Metadata { packages, .. } = cargo_metadata::MetadataCommand::new()
        .no_deps()
        .exec()
        .context("MetadataCommand::exec")?;
    let ebpf_package = packages
        .into_iter()
        .find(|cargo_metadata::Package { name, .. }| name == "golang-profiling-ebpf")
        .ok_or_else(|| anyhow!("golang-profiling-ebpf package not found"))?;
    aya_build::build_ebpf([ebpf_package])
        .context("Error building eBPF program")?;
    
    // Embed flamegraph.pl script
    let out_dir = env::var("OUT_DIR")?;
    let flamegraph_path = Path::new("../flamegraph.pl");
    
    if flamegraph_path.exists() {
        let flamegraph_content = fs::read_to_string(flamegraph_path)
            .context("Failed to read flamegraph.pl")?;
        
        let dest_path = Path::new(&out_dir).join("flamegraph.pl");
        fs::write(&dest_path, flamegraph_content)
            .context("Failed to write flamegraph.pl to OUT_DIR")?;
        
        println!("cargo:rerun-if-changed=../flamegraph.pl");
    }
    
    Ok(())
}
