# CHANGELOG

## [v0.3.0] - 2025-08-23

### Added
- **Off-CPU Profiling Support**
  - Implemented comprehensive off-CPU performance analysis functionality
  - Added `sched_switch` tracepoint handling for capturing process scheduling events
  - Support for mixed analysis mode collecting both on-CPU and off-CPU data simultaneously
  - Added timestamp recording and duration calculation capabilities

### Enhanced
- **eBPF Data Structure Optimization**
  - Extended `EbpfProfileKey` with `sample_type` field to distinguish on-CPU and off-CPU events
  - Added memory alignment padding `_padding: [0; 3]` ensuring cross-platform consistency
  - Optimized data aggregation logic to support multiple sampling types

### Testing & Validation
- **Comprehensive Test Programs**
  - Created `test_offcpu.go` specifically for off-CPU analysis testing
  - Created `test_mixed.go` for mixed analysis mode testing
  - Verified flame graph generation accuracy and completeness

### Technical Details
- **eBPF Program Enhancements**
  - Implemented `sched_switch` tracepoint hook functions
  - Added process state tracking and timestamp management
  - Optimized stack collection logic for different sampling scenarios

### Analysis & Problem Solving
- **Flame Graph Display Issues**
  - Analyzed sampling frequency impact on function visibility (default 99 Hz)
  - Identified why functions with longer off-CPU times may not appear in on-CPU flame graphs
  - Provided optimization recommendations for higher sampling frequency and extended analysis duration

### Memory Alignment Principles
- **eBPF Structure Alignment**
  - Detailed explanation of `_padding` field importance in eBPF programs
  - Elaborated on memory alignment impact on hardware access efficiency, eBPF verifier requirements, and cross-platform compatibility
  - Emphasized the importance of hash consistency when used as HashMap keys

---

## [v0.2.0] - 2025-08-22

### Added
- **PID Filtering Support**
  - Added precise PID filtering for targeted process profiling
  - Optimized eBPF map structures for better performance

### Fixed
- **eBPF Program Triggering Issues**
  - Fixed issue where idle processes (like etcd) wouldn't trigger perf events
  - Added proper activity detection for server applications

### Changed
- **Map Structure Optimization**
  - Changed `TARGET_PID` from HashMap to Array for better performance
  - Reduced memory usage: `STACK_TRACES` (16384→8192), `COUNTS` (40960→16384)

### Technical Details
- **Root Cause Analysis**
  - Server processes in idle state rarely trigger CPU-intensive operations
  - eBPF programs depend on perf events to collect stack information
  - Active workload is required for meaningful performance data collection

---

## [v0.1.0] - 2025-08-18

### Fixed
- **Go Symbol Resolution Issues**
  - Fixed symbol resolution for stripped Go binaries
  - Corrected PC address conversion logic in `.gopclntab` parsing

### Technical Background

#### Problem Description
When profiling Go programs with eBPF, flame graphs showed only hexadecimal addresses (e.g., `[unknown:0x4948c5]`) instead of readable function names, even though Go programs retain the `.gopclntab` section after being stripped.

#### Root Cause
The issue was caused by **double address conversion** in the PC address handling:

1. **First conversion** in `parse_pclntab_enhanced`: Converting relative PC to absolute PC
2. **Second conversion** in `resolve_pc`: Incorrectly converting absolute PC back to relative PC

#### Solution
Removed the erroneous address conversion logic in `resolve_pc` method:

```rust
// Before (Incorrect)
let lookup_pc = if gopclntab.version >= 18 {
    pc - gopclntab.text_start  // Wrong!
} else {
    pc - base_addr            // Wrong!
};

// After (Correct)
let lookup_pc = pc;  // Use absolute address directly
```

#### Results

**Before Fix:**
```
[unknown:0x4948c5] (2 samples, 0.63%)
[unknown:0x46e1a1] (1 samples, 0.31%)
[unknown:0x4415d8] (1 samples, 0.31%)
```

**After Fix:**
```
runtime.tgkill /usr/local/go/src/runtime/sys_linux_amd64.s:177
runtime.preemptone /usr/local/go/src/runtime/proc.go:6374
main::main /root/withlin/golang-profile/example/main.go:117
math::rand.(Rand).Intn /usr/local/go/src/math/rand/rand.go:183
```

### Key Technical Insights

#### Go Binary Structure
Go programs contain a special `.gopclntab` (Go Program Counter Line Table) section that includes:

- **Function table (functab)**: Stores PC addresses and metadata offsets for each function
- **Function name table (funcnametab)**: Stores all function name strings
- **File table (filetab)**: Stores source file paths
- **PC table (pctab)**: Stores PC to line number mappings

#### Version Compatibility
- **Go 1.2-1.16**: PC addresses relative to binary base address
- **Go 1.18+**: Introduced `text_start` field, PC addresses relative to text segment start

### Files Modified
- `golang-profiling/src/symbol_resolver.rs`
  - Fixed PC address conversion logic in `resolve_pc` method
  - Simplified function table construction in `parse_pclntab_enhanced`

### Impact
This fix significantly improves the readability and usability of performance analysis for Go programs, enabling proper symbol resolution even for stripped binaries.

---

## Development Notes

### Testing
- Verified with `test_app_active` (stripped binary)
- Supports both Go 1.18+ and earlier versions
- Flame graphs now correctly display function names and source locations

### Best Practices
1. **Understand data structures**: Deep understanding of `.gopclntab` internal structure
2. **Address space management**: Maintain consistency in address conversions
3. **Version compatibility**: Consider differences across Go versions
4. **Debug-driven development**: Use detailed logging for problem identification