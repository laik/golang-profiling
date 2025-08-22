use anyhow::{anyhow, Result};
use aya::{
    maps::{Array, MapData, HashMap as AyaHashMap},
    programs::{PerfEvent, TracePoint, perf_event},
    util::online_cpus,
    Ebpf,
};
use aya_log::EbpfLogger;
use clap::Parser;
use golang_profiling_common::{
    GoRuntimeInfo, ProfileKey, EbpfProfileKey,
};
use log::{info, warn, error};
use std::{
    collections::HashMap,
    convert::TryInto,
    path::PathBuf,
    process,
    sync::{Arc, Mutex, atomic::{AtomicU64, Ordering}},
    time::Duration,
    fs,
    io::Write,
};
use tokio::{signal, time};

// Embed the flamegraph.pl script at compile time
const FLAMEGRAPH_SCRIPT: &str = include_str!("../../flamegraph.pl");

mod golang_parser;
mod symbol_resolver;
mod flamegraph_export;
mod dwarf_parser;
mod elfgopclntab;

use golang_parser::GoRuntimeParser;
use symbol_resolver::SymbolResolver;
use flamegraph_export::FlameGraphExporter;
use dwarf_parser::DwarfParser;

#[derive(Parser, Debug)]
#[command(name = "golang-profile")]
#[command(about = "High-performance Golang CPU profiler with flame graph generation")]
struct Args {
    /// Target process ID to profile
    #[arg(short, long)]
    pid: Option<u32>,
    
    /// Target process name to profile
    #[arg(short = 'n', long)]
    process_name: Option<String>,
    
    /// Duration to profile in seconds
    #[arg(short, long, default_value = "5")]
    duration: u64,
    
    /// Output file for flame graph (SVG format)
    #[arg(short, long, default_value = "flamegraph.svg")]
    output: PathBuf,
    
    /// Enable off-CPU profiling
    #[arg(long)]
    off_cpu: bool,
    
    /// Sampling frequency in Hz
    #[arg(short, long, default_value = "99")]
    frequency: u64,
    
    /// Verbose logging
    #[arg(short, long)]
    verbose: bool,
    
    /// Export folded stacks for external tools
    #[arg(long)]
    export_folded: Option<PathBuf>,
    
    /// Flame graph title
    #[arg(long, default_value = "Golang CPU Profile")]
    title: String,
    
    /// Flame graph subtitle
    #[arg(long)]
    subtitle: Option<String>,
    
    /// Color palette: hot, mem, io, wakeup, chain, java, js, perl, red, green, blue, aqua, yellow, purple, orange
    #[arg(long, default_value = "kernel_user")]
    colors: String,
    
    /// Background colors: yellow, blue, green, grey, or custom hex color
    #[arg(long)]
    bgcolors: Option<String>,
    
    /// Image width in pixels
    #[arg(long, default_value = "1200")]
    width: u32,
    
    /// Frame height in pixels
    #[arg(long, default_value = "16")]
    height: u32,
    
    /// Font type
    #[arg(long, default_value = "Verdana")]
    fonttype: String,
    
    /// Font size
    #[arg(long, default_value = "12")]
    fontsize: f32,
    
    /// Generate inverted icicle graph
    #[arg(long)]
    inverted: bool,
    
    /// Generate flame chart (sort by time, do not merge stacks)
    #[arg(long)]
    flamechart: bool,
    
    /// Use hash-based colors
    #[arg(long)]
    hash: bool,
    
    /// Use random colors
    #[arg(long)]
    random: bool
}

struct ProfilerState {
    aggregated_counts: Arc<Mutex<HashMap<ProfileKey, u64>>>,
    symbol_resolver: Arc<Mutex<SymbolResolver>>,
    stack_traces_map: Arc<Mutex<Option<aya::maps::StackTraceMap<MapData>>>>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let args = Args::parse();
    
    if args.verbose {
        env_logger::Builder::from_default_env()
            .filter_level(log::LevelFilter::Debug)
            .init();
    } else {
        env_logger::init();
    }
    
    info!("Starting Golang profiler...");
    
    // Determine target PID
    let target_pid = match (args.pid, args.process_name.as_ref()) {
        (Some(pid), _) => pid,
        (None, Some(name)) => find_process_by_name(name)?,
        (None, None) => {
            error!("Either --pid or --process-name must be specified");
            process::exit(1);
        }
    };
    
    info!("Profiling process PID: {}", target_pid);
    
    // Bump the memlock rlimit
    let rlim = libc::rlimit {
        rlim_cur: libc::RLIM_INFINITY,
        rlim_max: libc::RLIM_INFINITY,
    };
    let ret = unsafe { libc::setrlimit(libc::RLIMIT_MEMLOCK, &rlim) };
    if ret != 0 {
        warn!("Failed to remove limit on locked memory: {}", ret);
    }
    
    // Load eBPF program
    let mut ebpf = Ebpf::load(aya::include_bytes_aligned!(concat!(
        env!("OUT_DIR"),
        "/golang-profiling"
    )))?;
    
    if let Err(e) = EbpfLogger::init(&mut ebpf) {
        warn!("Failed to initialize eBPF logger: {}", e);
    }
    
    // Parse Go runtime information
    let mut go_parser = GoRuntimeParser::new();
    let runtime_info = go_parser.parse_process(target_pid)?;
    info!("Detected Go runtime version: {:?}", 
          String::from_utf8_lossy(&runtime_info.version));
    
    // Set target PID in eBPF map for filtering
    let mut target_pid_map: Array<_, u32> = 
        Array::try_from(ebpf.map_mut("TARGET_PID").unwrap())?;
    target_pid_map.set(0, target_pid, 0)?;
    info!("Target PID {} configured for eBPF filtering", target_pid);
    
    // Verify the PID was set correctly
    if let Ok(stored_pid) = target_pid_map.get(&0, 0) {
        info!("Verified: TARGET_PID map contains: {}", stored_pid);
    } else {
        error!("Failed to verify TARGET_PID map setting");
    }
    
    // Runtime info is now only used in user space for symbol resolution
    
    // Initialize symbol resolver
    let symbol_resolver = Arc::new(Mutex::new(
        SymbolResolver::new(target_pid, runtime_info)?
    ));
    
    // Initialize profiler state
    let state = Arc::new(ProfilerState {
        aggregated_counts: Arc::new(Mutex::new(HashMap::new())),
        symbol_resolver: symbol_resolver.clone(),
        stack_traces_map: Arc::new(Mutex::new(None)),
    });
    
    // Attach perf event for on-CPU profiling
    let program: &mut PerfEvent = ebpf.program_mut("golang_profile").unwrap().try_into()?;
    program.load()?;
    
    for cpu in online_cpus().map_err(|(_, error)| error)? {
        program.attach(
            perf_event::PerfTypeId::Software,
            perf_event::perf_sw_ids::PERF_COUNT_SW_CPU_CLOCK as u64,
            perf_event::PerfEventScope::AllProcessesOneCpu { cpu },
            perf_event::SamplePolicy::Frequency(args.frequency),
            true,
        )?;
    }
    
    // Attach tracepoint for off-CPU profiling if enabled
    if args.off_cpu {
        let program: &mut TracePoint = ebpf.program_mut("sched_switch").unwrap().try_into()?;
        program.load()?;
        program.attach("sched", "sched_switch")?;
        info!("Off-CPU profiling enabled");
    }
    
    // Get the COUNTS map for reading aggregated data
    let counts_map: AyaHashMap<_, EbpfProfileKey, u64> = 
        AyaHashMap::try_from(ebpf.take_map("COUNTS").unwrap())?;
    
    // Get the STACK_TRACES map for stack resolution
    let stack_traces_map: aya::maps::StackTraceMap<_> = 
        aya::maps::StackTraceMap::try_from(ebpf.take_map("STACK_TRACES").unwrap())?;
    
    // Store the stack_traces_map in the state
    *state.stack_traces_map.lock().unwrap() = Some(stack_traces_map);
    
    let state_clone = state.clone();
    
    tokio::spawn(async move {
        read_aggregated_counts(counts_map, state_clone).await;
    });
    
    // Wait for specified duration or Ctrl-C
    info!("Profiling for {} seconds... Press Ctrl-C to stop early", args.duration);
    
    tokio::select! {
        _ = time::sleep(Duration::from_secs(args.duration)) => {
            info!("Profiling duration completed");
        }
        _ = signal::ctrl_c() => {
            info!("Received Ctrl-C, stopping profiler");
        }
    }
    
    // Generate flame graph using Brendan Gregg's FlameGraph tools
    info!("Generating flame graph...");
    let aggregated_counts = state.aggregated_counts.lock().unwrap().clone();
    let resolver = state.symbol_resolver.lock().unwrap();
    
    // Get the STACK_TRACES map from state
    let stack_traces_map_guard = state.stack_traces_map.lock().unwrap();
    let stack_traces_map = stack_traces_map_guard.as_ref().unwrap();
    
    // Convert aggregated data to format compatible with FlameGraph tools
    let mut converted_data = HashMap::new();
    
    for (profile_key, count) in &aggregated_counts {
        // Get stack traces for this profile key
        let mut stack = Vec::new();
        
        // Add kernel stack if present
         if profile_key.kernel_stack_id >= 0 {
             if let Ok(kernel_stack) = stack_traces_map.get(&(profile_key.kernel_stack_id as u32), 0) {
                 for frame in kernel_stack.frames().iter().rev() {
                     if frame.ip != 0 {
                         stack.push(frame.ip);
                     }
                 }
             }
         }
         
         // Add user stack if present
         if profile_key.user_stack_id >= 0 {
             if let Ok(user_stack) = stack_traces_map.get(&(profile_key.user_stack_id as u32), 0) {
                 for frame in user_stack.frames().iter().rev() {
                     if frame.ip != 0 {
                         stack.push(frame.ip);
                     }
                 }
             }
         }
        
        if !stack.is_empty() {
            converted_data.insert(stack, *count);
        }
    }
    
    let exporter = FlameGraphExporter::new()?;
    
    // Export folded stacks if requested
    if let Some(folded_path) = &args.export_folded {
        exporter.export_folded_stacks(&converted_data, folded_path, &*resolver)?;
        info!("Folded stacks exported to: {}", folded_path.display());
    }
    
    // Generate flame graph using Brendan Gregg's tools
    let folded_file = "stacks.folded";
    exporter.export_folded_stacks(&converted_data, std::path::Path::new(folded_file), &*resolver)?;
    
    // Use embedded flamegraph.pl script to generate SVG
    let temp_script_path = "/tmp/flamegraph_embedded.pl";
    fs::write(temp_script_path, FLAMEGRAPH_SCRIPT)?;
    
    let mut cmd = std::process::Command::new("perl");
    cmd.arg(temp_script_path);
    
    // Add custom parameters
    cmd.arg("--title").arg(&args.title);
    
    if let Some(subtitle) = &args.subtitle {
        cmd.arg("--subtitle").arg(subtitle);
    }
    
    cmd.arg("--colors").arg(&args.colors);
    
    if let Some(bgcolors) = &args.bgcolors {
        cmd.arg("--bgcolors").arg(bgcolors);
    }
    
    cmd.arg("--width").arg(args.width.to_string());
    cmd.arg("--height").arg(args.height.to_string());
    cmd.arg("--fonttype").arg(&args.fonttype);
    cmd.arg("--fontsize").arg(args.fontsize.to_string());
    
    if args.inverted {
        cmd.arg("--inverted");
    }
    
    if args.flamechart {
        cmd.arg("--flamechart");
    }
    
    if args.hash {
        cmd.arg("--hash");
    }
    
    if args.random {
        cmd.arg("--random");
    }
    
    cmd.arg(folded_file);
    
    let output = cmd.output()?;
    
    // Clean up temporary script file
    let _ = fs::remove_file(temp_script_path);
    
    if !output.status.success() {
        anyhow::bail!("Failed to generate flame graph with embedded flamegraph.pl: {}", 
            String::from_utf8_lossy(&output.stderr));
    }
    
    std::fs::write(&args.output, output.stdout)?;
    
    // Clean up temporary file
    let _ = std::fs::remove_file(folded_file);
    
    info!("Flame graph saved to: {}", args.output.display());
    
    info!("Total samples collected: {}", aggregated_counts.len());
    
    Ok(())
}

async fn read_aggregated_counts(
    counts_map: AyaHashMap<MapData, EbpfProfileKey, u64>, 
    state: Arc<ProfilerState>
) {
    loop {
        // Read all entries from the COUNTS map (now filtered by target PID in eBPF)
        let mut current_counts = HashMap::new();
        let mut total_samples = 0u64;
        
        // Iterate through all entries in the eBPF map
        for item in counts_map.iter() {
            if let Ok((ebpf_key, value)) = item {
                // Convert EbpfProfileKey to ProfileKey
                let profile_key = ProfileKey {
                    pid: ebpf_key.pid,
                    kernel_ip: 0,
                    user_stack_id: ebpf_key.user_stack_id,
                    kernel_stack_id: ebpf_key.kernel_stack_id,
                    name: [0u8; 16],
                };
                
                current_counts.insert(profile_key, value);
                total_samples += value;
            }
        }
        
        // Update the aggregated counts in our state
        if !current_counts.is_empty() {
            let mut aggregated = state.aggregated_counts.lock().unwrap();
            *aggregated = current_counts;
            
            // Log sampling progress every 10 seconds (100 iterations * 100ms)
            static ITERATION_COUNT: AtomicU64 = AtomicU64::new(0);
            let count = ITERATION_COUNT.fetch_add(1, Ordering::Relaxed);
            if count % 100 == 0 {
                info!("Collected {} samples from target PID (eBPF filtered)", total_samples);
            }
        } else {
            // Log when no data is collected
            static NO_DATA_COUNT: AtomicU64 = AtomicU64::new(0);
            let count = NO_DATA_COUNT.fetch_add(1, Ordering::Relaxed);
            if count % 50 == 0 {
                warn!("No stack data collected after {} iterations. eBPF program may not be triggering.", count);
            }
        }
        
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}



fn find_process_by_name(name: &str) -> anyhow::Result<u32> {
    let output = std::process::Command::new("pgrep")
        .arg("-f")
        .arg(name)
        .output()?;
    
    if !output.status.success() {
        anyhow::bail!("Process '{}' not found", name);
    }
    
    let stdout = String::from_utf8(output.stdout)?;
    let pid = stdout.trim().lines().next()
        .ok_or_else(|| anyhow::anyhow!("No PID found for process '{}'", name))?
        .parse::<u32>()?;
    
    Ok(pid)
}
