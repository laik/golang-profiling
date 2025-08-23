#![no_std]
#![no_main]

use aya_ebpf::{
    helpers::{bpf_get_current_pid_tgid, bpf_ktime_get_ns},
    macros::{map, perf_event, tracepoint},
    maps::{Array, HashMap, StackTrace},
    programs::{PerfEventContext, TracePointContext},
};
use aya_log_ebpf::info;
use golang_profiling_common::{EbpfProfileKey, SAMPLE_TYPE_ON_CPU, SAMPLE_TYPE_OFF_CPU};
use aya_ebpf::helpers::bpf_probe_read_user;

// eBPF program metadata
#[cfg(not(test))]
#[unsafe(no_mangle)]
static _license: &[u8] = b"GPL";

#[cfg(not(test))]
#[unsafe(no_mangle)]
static _version: u32 = 0xFFFFFFFE;

// BPF constants
const BPF_F_USER_STACK: u64 = 1 << 8;

// Optimized map sizes for better memory usage
// Stack trace storage - reduced from 16384 to 8192 for memory efficiency
#[map]
static STACK_TRACES: StackTrace = StackTrace::with_max_entries(8192, 0);

// Aggregated counts - reduced from 40960 to 16384 for single PID profiling
#[map]
static COUNTS: HashMap<EbpfProfileKey, u64> = HashMap::with_max_entries(16384, 0);

// Target PID configuration - using Array for single value storage
#[map]
static TARGET_PID: Array<u32> = Array::with_max_entries(1, 0);

// Process timestamps for off-CPU duration calculation
#[map]
static PROCESS_TIMESTAMPS: HashMap<u32, u64> = HashMap::with_max_entries(4096, 0);

#[perf_event]
pub fn golang_profile(ctx: PerfEventContext) -> u32 {
    match unsafe { try_golang_profile(ctx) } {
        Ok(ret) => ret,
        Err(ret) => ret,
    }
}

unsafe fn try_golang_profile(ctx: PerfEventContext) -> Result<u32, u32> {
    let pid_tgid = bpf_get_current_pid_tgid();
    let tgid = (pid_tgid >> 32) as u32;

    // Skip idle process (PID 0)
    if tgid == 0 {
        return Ok(0);
    }
    

    // Check if we have a target PID configured and filter accordingly
    let target_pid = match TARGET_PID.get(0) {
        Some(pid) => pid,
        None => {
            // No target PID configured, skip profiling
            return Ok(0);
        }
    };

    // Debug output disabled for production use
    // info!(&ctx, "Current PID: {}, Target PID: {}", tgid, *target_pid);

    // If target_pid is 0, it means no filtering (profile all processes)
    // Otherwise, only profile the specified PID
    if *target_pid != 0 && tgid != *target_pid {
        return Ok(0);
    }

    // Get stack traces
    let user_stack_id = STACK_TRACES
        .get_stackid(&ctx, BPF_F_USER_STACK)
        .unwrap_or(-1) as i32;

    let kernel_stack_id = STACK_TRACES.get_stackid(&ctx, 0).unwrap_or(-1) as i32;

    // Create profile key
    let key = EbpfProfileKey {
        pid: tgid,
        user_stack_id,
        kernel_stack_id,
        sample_type: SAMPLE_TYPE_ON_CPU,
        _padding: [0; 3],
    };

    // Increment count
    let count = COUNTS.get(&key).copied().unwrap_or(0);
    let _ = COUNTS.insert(&key, &(count + 1), 0);

    Ok(0)
}

#[tracepoint]
pub fn sched_switch(ctx: TracePointContext) -> u32 {
    match unsafe { try_sched_switch(ctx) } {
        Ok(ret) => ret,
        Err(ret) => ret,
    }
}

unsafe fn try_sched_switch(ctx: TracePointContext) -> Result<u32, u32> {
    let pid_tgid = bpf_get_current_pid_tgid();
    let prev_pid = (pid_tgid >> 32) as u32;
    
    // Skip idle process (PID 0)
    if prev_pid == 0 {
        return Ok(0);
    }
    
    // Check if we have a target PID configured and filter accordingly
    let target_pid = match TARGET_PID.get(0) {
        Some(pid) => pid,
        None => {
            // No target PID configured, skip profiling
            return Ok(0);
        }
    };
    
    // If target_pid is 0, it means no filtering (profile all processes)
    // Otherwise, only profile the specified PID
    if *target_pid != 0 && prev_pid != *target_pid {
        return Ok(0);
    }
    
    let current_time = bpf_ktime_get_ns();
    
    // Check if we have a previous timestamp for this process
    if let Some(prev_time) = PROCESS_TIMESTAMPS.get(&prev_pid) {
        let duration = current_time - *prev_time;
        
        // Only record if duration is significant (> 1ms)
        if duration > 1_000_000 {
            // Get stack traces for the process being switched out
            let user_stack_id = STACK_TRACES
                .get_stackid(&ctx, BPF_F_USER_STACK)
                .unwrap_or(-1) as i32;
            
            let kernel_stack_id = STACK_TRACES.get_stackid(&ctx, 0).unwrap_or(-1) as i32;
            
            // Create profile key for off-CPU event
            let key = EbpfProfileKey {
                pid: prev_pid,
                user_stack_id,
                kernel_stack_id,
                sample_type: SAMPLE_TYPE_OFF_CPU,
                _padding: [0; 3],
            };
            
            // Use duration in microseconds as count (to avoid overflow)
            let duration_us = (duration / 1000) as u64;
            let count = COUNTS.get(&key).copied().unwrap_or(0);
            let _ = COUNTS.insert(&key, &(count + duration_us), 0);
        }
        
        // Remove the timestamp entry
        let _ = PROCESS_TIMESTAMPS.remove(&prev_pid);
    }
    
    // Store timestamp for when this process gets scheduled in
    let _ = PROCESS_TIMESTAMPS.insert(&prev_pid, &current_time, 0);
    
    Ok(0)
}

// Optimized for aya framework - minimal eBPF program

#[cfg(not(test))]
#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    loop {}
}
