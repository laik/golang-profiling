use crate::symbol_resolver::SymbolResolver;
use anyhow::Result;
use std::collections::HashMap;
use std::fs::File;
use std::io::{self, Write};
use std::path::Path;
use std::time::{SystemTime, UNIX_EPOCH};

/// Export performance data in formats compatible with external flame graph tools
pub struct FlameGraphExporter {
    // We'll create the symbol resolver when needed since it requires runtime info
}

impl FlameGraphExporter {
    pub fn new() -> Result<Self> {
        Ok(FlameGraphExporter {})
    }

    /// Export data in Brendan Gregg's FlameGraph format (folded stacks)
    pub fn export_folded_stacks(
        &self,
        aggregated_data: &HashMap<Vec<u64>, u64>,
        output_path: &Path,
        symbol_resolver: &SymbolResolver,
    ) -> Result<()> {
        let mut file = File::create(output_path)
            .map_err(|e| anyhow::anyhow!("Failed to create file: {}", e))?;

        for (stack, count) in aggregated_data {
            let mut stack_str = String::new();

            // Build the stack trace string in reverse order (leaf to root)
            for (i, &pc) in stack.iter().rev().enumerate() {
                if i > 0 {
                    stack_str.push(';');
                }

                let symbol = symbol_resolver.resolve_pc(pc);
                stack_str.push_str(&symbol);
            }

            // Write the folded stack line: "stack_trace count"
            writeln!(file, "{} {}", stack_str, count)
                .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        }

        Ok(())
    }

    /// Export data in perf script format
    pub fn export_perf_script(
        &self,
        aggregated_data: &HashMap<Vec<u64>, u64>,
        output_path: &Path,
        symbol_resolver: &SymbolResolver,
    ) -> Result<()> {
        let mut file = File::create(output_path)
            .map_err(|e| anyhow::anyhow!("Failed to create file: {}", e))?;

        // Write perf script header
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        writeln!(file, "# captured on: {}", timestamp)
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# hostname : localhost")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# os release : Linux")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# perf version : simulated")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# arch : x86_64")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# nrcpus online : 1")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# nrcpus avail : 1")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# cpudesc : Unknown")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# total memory : Unknown")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "# cmdline : golang-profile")
            .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
        writeln!(file, "#").map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;

        let mut sample_id = 1;

        for (stack, count) in aggregated_data {
            // Simulate multiple samples for the count
            for _ in 0..*count {
                writeln!(
                    file,
                    "golang-profile {} [000] {:.6}: cycles:",
                    sample_id,
                    sample_id as f64 / 1000000.0
                )
                .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;

                // Write stack trace (from leaf to root)
                for &pc in stack.iter().rev() {
                    let symbol = symbol_resolver.resolve_pc(pc);
                    writeln!(file, "\t{:016x} {}", pc, symbol)
                        .map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?;
                }

                writeln!(file).map_err(|e| anyhow::anyhow!("Failed to write to file: {}", e))?; // Empty line between samples
                sample_id += 1;
            }
        }

        Ok(())
    }
}
