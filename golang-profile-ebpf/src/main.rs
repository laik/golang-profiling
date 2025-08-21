#![no_std]
#![no_main]

use aya_ebpf::{
    bindings::BPF_F_CURRENT_CPU,
    helpers::{bpf_get_current_pid_tgid, bpf_get_current_comm},
    macros::{map, perf_event},
    maps::{HashMap, PerCpuArray, PerfEventArray, StackTrace},
    programs::PerfEventContext,
    EbpfContext,
};
use golang_profile_common::{EbpfProfileKey, SampleEvent, StackFrame, GoRuntimeInfo, SAMPLE_TYPE_ON_CPU, MAX_STACK_DEPTH};

mod vmlinux;

// BPF constants
const BPF_F_USER_STACK: u64 = 1 << 8;

// Stack trace storage - similar to BCC's BPF_STACK_TRACE
#[map]
static STACK_TRACES: StackTrace = StackTrace::with_max_entries(16384, 0);

// Aggregated counts - using eBPF profile key
#[map]
static COUNTS: HashMap<EbpfProfileKey, u64> = HashMap::with_max_entries(40960, 0);

// Events for sending aggregated data to userspace
#[map]
static EVENTS: PerfEventArray<SampleEvent> = PerfEventArray::new(0);

#[map]
static GO_RUNTIME_INFO: HashMap<u32, GoRuntimeInfo> = HashMap::with_max_entries(1024, 0);

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
    let _pid = pid_tgid as u32;
    
    // Skip idle process (PID 0) by default
    if tgid == 0 {
        return Ok(0);
    }
    
    // Get stack traces
    let user_stack_id = unsafe {
        STACK_TRACES.get_stackid(&ctx, BPF_F_USER_STACK)
    }.unwrap_or(-1) as i32;
    
    let kernel_stack_id = unsafe {
        STACK_TRACES.get_stackid(&ctx, 0)
    }.unwrap_or(-1) as i32;
    
    // Create profile key with stack information
    let key = EbpfProfileKey {
        pid: tgid,
        user_stack_id,
        kernel_stack_id,
    };
    
    // For now, just count by PID to avoid complex stack operations
    // In a production version, we would implement more sophisticated aggregation
    
    // Increment count for this process
    let count = COUNTS.get(&key).copied().unwrap_or(0);
    let _ = COUNTS.insert(&key, &(count + 1), 0);
    
    Ok(0)
}

// Stack unwinding is now handled in userspace
// eBPF only collects basic sampling data

// Removed complex goroutine functions to reduce eBPF program size

#[cfg(not(test))]
#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    loop {}
}
