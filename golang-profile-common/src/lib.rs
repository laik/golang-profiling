#![cfg_attr(not(feature = "user"), no_std)]



/// Maximum number of stack frames to capture
pub const MAX_STACK_DEPTH: usize = 127;

/// Maximum length of function name
pub const MAX_FUNC_NAME_LEN: usize = 256;

/// Stack frame information
#[repr(C)]
#[derive(Clone, Copy)]
pub struct StackFrame {
    /// Program counter (instruction pointer)
    pub pc: u64,
    /// Stack pointer
    pub sp: u64,
    /// Function entry point
    pub func_entry: u64,
}

/// Sample event sent from eBPF to userspace
#[repr(C)]
#[derive(Clone, Copy)]
pub struct SampleEvent {
    /// Process ID
    pub pid: u32,
    /// Thread ID
    pub tid: u32,
    /// CPU ID
    pub cpu: u32,
    /// Timestamp in nanoseconds
    pub timestamp: u64,
    /// Sample type (on-cpu or off-cpu)
    pub sample_type: u8,
    /// Number of valid stack frames
    pub stack_depth: u8,
    /// Stack frames
    pub stack: [StackFrame; MAX_STACK_DEPTH],
    /// Duration for off-cpu events (in nanoseconds)
    pub duration: u64,
}

/// Sample types
pub const SAMPLE_TYPE_ON_CPU: u8 = 1;
pub const SAMPLE_TYPE_OFF_CPU: u8 = 2;

/// Simplified profile key for eBPF to avoid verification issues
#[repr(C)]
#[derive(Clone, Copy, Debug, Hash, PartialEq, Eq)]
pub struct EbpfProfileKey {
    /// Process ID
    pub pid: u32,
    /// User stack ID from stack trace map
    pub user_stack_id: i32,
    /// Kernel stack ID from stack trace map
    pub kernel_stack_id: i32,
}

/// Complete profile aggregation key for userspace processing
#[repr(C)]
#[derive(Clone, Copy, Debug, Hash, PartialEq, Eq)]
pub struct ProfileKey {
    /// Process ID
    pub pid: u32,
    /// Kernel instruction pointer
    pub kernel_ip: u64,
    /// User stack ID from stack trace map
    pub user_stack_id: i32,
    /// Kernel stack ID from stack trace map
    pub kernel_stack_id: i32,
    /// Process name (TASK_COMM_LEN = 16)
    pub name: [u8; 16],
}

/// Golang runtime information
#[repr(C)]
#[derive(Clone, Copy, Debug, Default)]
pub struct GoRuntimeInfo {
    /// Go version
    pub version: [u8; 32],
    /// Goroutine struct offset
    pub g_offset: u64,
    /// Stack base offset in goroutine struct
    pub stack_base_offset: u64,
    /// Stack size offset in goroutine struct
    pub stack_size_offset: u64,
    /// Function table base address
    pub func_tab_base: u64,
    /// Function table size
    pub func_tab_size: u64,
    /// PC quantum (instruction alignment)
    pub pc_quantum: u32,
}

/// Function information for symbol resolution
#[repr(C)]
#[derive(Clone, Copy, Debug)]
pub struct FuncInfo {
    /// Function entry point
    pub entry: u64,
    /// Function name offset in string table
    pub name_off: u32,
    /// Function file name offset
    pub file_off: u32,
    /// Function line number
    pub line: u32,
}

#[cfg(feature = "user")]
pub mod user {
    extern crate std;
    use super::*;
    use std::collections::HashMap;
    use std::string::String;
    use std::vec::Vec;
    
    /// Flame graph node
    #[derive(Debug, Clone)]
    pub struct FlameNode {
        pub name: String,
        pub value: u64,
        pub children: std::collections::HashMap<String, FlameNode>,
    }
    
    impl FlameNode {
        pub fn new(name: String) -> Self {
            Self {
                name,
                value: 0,
                children: std::collections::HashMap::new(),
            }
        }
        
        pub fn add_sample(&mut self, stack: &[String], value: u64) {
            self.value += value;
            if let Some((first, rest)) = stack.split_first() {
                let child = self.children.entry(first.clone()).or_insert_with(|| {
                    FlameNode::new(first.clone())
                });
                child.add_sample(rest, value);
            }
        }
        
        pub fn to_folded(&self, prefix: &str) -> Vec<String> {
            let mut result = Vec::new();
            let current_name = if prefix.is_empty() {
                self.name.clone()
            } else {
                format!("{};{}", prefix, self.name)
            };
            
            if self.children.is_empty() {
                result.push(format!("{} {}", current_name, self.value));
            } else {
                for child in self.children.values() {
                    result.extend(child.to_folded(&current_name));
                }
            }
            result
        }
    }
}

// eBPF compatibility
#[cfg(feature = "user")]
unsafe impl aya::Pod for SampleEvent {}
#[cfg(feature = "user")]
unsafe impl aya::Pod for StackFrame {}
#[cfg(feature = "user")]
unsafe impl aya::Pod for GoRuntimeInfo {}
#[cfg(feature = "user")]
unsafe impl aya::Pod for FuncInfo {}
#[cfg(feature = "user")]
unsafe impl aya::Pod for ProfileKey {}
#[cfg(feature = "user")]
unsafe impl aya::Pod for EbpfProfileKey {}