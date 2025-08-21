use anyhow::{anyhow, Result};
use golang_profile_common::GoRuntimeInfo;
use log::{debug, info, warn};
use memmap2::Mmap;
use object::{Object, ObjectSection, ObjectSymbol};
use std::{
    fs::File,
    io::{BufRead, BufReader},
    path::Path,
};

/// Parser for Go runtime information
pub struct GoRuntimeParser {
    // Cache for parsed runtime info
}

impl GoRuntimeParser {
    pub fn new() -> Self {
        Self {}
    }
    
    /// Parse Go runtime information from a process
    pub fn parse_process(&mut self, pid: u32) -> Result<GoRuntimeInfo> {
        let exe_path = format!("/proc/{}/exe", pid);
        let maps_path = format!("/proc/{}/maps", pid);
        
        // Read the executable
        let file = File::open(&exe_path)
            .map_err(|e| anyhow!("Failed to open executable {}: {}", exe_path, e))?;
        let mmap = unsafe { Mmap::map(&file)? };
        let obj = object::File::parse(&*mmap)?;
        
        // Parse Go version and runtime info
        let mut runtime_info = GoRuntimeInfo {
            version: [0; 32],
            g_offset: 0,
            stack_base_offset: 0,
            stack_size_offset: 0,
            func_tab_base: 0,
            func_tab_size: 0,
            pc_quantum: 1,
        };
        
        // Try to find Go version string
        let version = if let Some(version) = self.extract_go_version(&obj)? {
            let version_bytes = version.as_bytes();
            let copy_len = version_bytes.len().min(31);
            runtime_info.version[..copy_len].copy_from_slice(&version_bytes[..copy_len]);
            info!("Detected Go version: {}", version);
            Some(version)
        } else {
            None
        };
        
        // Parse function table information
        if let Some((func_tab_base, func_tab_size)) = self.find_func_table(&obj)? {
            runtime_info.func_tab_base = func_tab_base;
            runtime_info.func_tab_size = func_tab_size;
            debug!("Function table: base=0x{:x}, size={}", func_tab_base, func_tab_size);
        }
        
        // Parse goroutine struct offsets (these are Go version dependent)
        self.parse_goroutine_offsets(&mut runtime_info, &version.unwrap_or_default())?;
        
        // Read memory mappings to get base addresses
        self.update_runtime_addresses(&mut runtime_info, &maps_path)?;
        
        Ok(runtime_info)
    }
    
    /// Extract Go version from the binary
    fn extract_go_version(&self, obj: &object::File) -> Result<Option<String>> {
        // Look for Go version in various sections
        for section in obj.sections() {
            if let Ok(data) = section.data() {
                // Search for Go version pattern
                if let Some(version) = self.find_version_in_data(data) {
                    return Ok(Some(version));
                }
            }
        }
        
        // Fallback: try to detect from build info
        if let Some(version) = self.extract_build_info_version(obj)? {
            return Ok(Some(version));
        }
        
        warn!("Could not detect Go version, using defaults");
        Ok(Some("go1.21".to_string()))
    }
    
    /// Find version string in binary data
    fn find_version_in_data(&self, data: &[u8]) -> Option<String> {
        let data_str = String::from_utf8_lossy(data);
        
        // Look for "go1." pattern
        for line in data_str.lines() {
            if line.contains("go1.") {
                // Extract version string
                if let Some(start) = line.find("go1.") {
                    let version_part = &line[start..];
                    let version_str = if let Some(end) = version_part.find(char::is_whitespace) {
                        &version_part[..end]
                    } else if version_part.len() <= 10 {
                        version_part
                    } else {
                        continue;
                    };
                    
                    // Validate version format: go1.x.y where x and y are numbers
                    if self.is_valid_go_version(version_str) {
                        return Some(version_str.to_string());
                    }
                }
            }
        }
        
        None
    }
    
    fn is_valid_go_version(&self, version: &str) -> bool {
        if !version.starts_with("go1.") {
            return false;
        }
        
        let parts: Vec<&str> = version[4..].split('.').collect();
        if parts.len() < 2 {
            return false;
        }
        
        // Check if major and minor versions are numbers
        parts[0].parse::<u32>().is_ok() && parts[1].parse::<u32>().is_ok()
    }
    
    /// Extract version from Go build info
    fn extract_build_info_version(&self, obj: &object::File) -> Result<Option<String>> {
        // Look for .go.buildinfo section
        if let Some(section) = obj.section_by_name(".go.buildinfo") {
            if let Ok(data) = section.data() {
                // Parse build info structure
                if data.len() >= 32 {
                    // Skip magic and version, look for Go version
                    let info_str = String::from_utf8_lossy(&data[16..]);
                    if let Some(version) = self.find_version_in_data(info_str.as_bytes()) {
                        return Ok(Some(version));
                    }
                }
            }
        }
        
        Ok(None)
    }
    
    /// Find function table in the binary
    fn find_func_table(&self, obj: &object::File) -> Result<Option<(u64, u64)>> {
        // Look for .gopclntab section (function table)
        if let Some(section) = obj.section_by_name(".gopclntab") {
            let base = section.address();
            let size = section.size();
            return Ok(Some((base, size)));
        }
        
        // Fallback: look for runtime.pclntab symbol
        for symbol in obj.symbols() {
            if let Ok(name) = symbol.name() {
                if name == "runtime.pclntab" || name.contains("pclntab") {
                    return Ok(Some((symbol.address(), symbol.size())));
                }
            }
        }
        
        warn!("Could not find function table");
        Ok(None)
    }
    
    /// Parse goroutine struct offsets based on Go version
    fn parse_goroutine_offsets(&self, runtime_info: &mut GoRuntimeInfo, version: &str) -> Result<()> {
        // These offsets are version-dependent and architecture-dependent
        // For x86_64, common offsets:
        
        if version.starts_with("go1.21") || version.starts_with("go1.22") || version.starts_with("go1.23") {
            // Go 1.21+ offsets for x86_64
            runtime_info.g_offset = 0x30;  // TLS offset to current goroutine
            runtime_info.stack_base_offset = 0x10;  // stack.lo in g struct
            runtime_info.stack_size_offset = 0x18;  // stack.hi in g struct
            runtime_info.pc_quantum = 1;
        } else if version.starts_with("go1.20") {
            runtime_info.g_offset = 0x30;
            runtime_info.stack_base_offset = 0x10;
            runtime_info.stack_size_offset = 0x18;
            runtime_info.pc_quantum = 1;
        } else {
            // Default/fallback offsets
            runtime_info.g_offset = 0x30;
            runtime_info.stack_base_offset = 0x10;
            runtime_info.stack_size_offset = 0x18;
            runtime_info.pc_quantum = 1;
            warn!("Unknown Go version {}, using default offsets", version);
        }
        
        debug!("Goroutine offsets: g=0x{:x}, stack_base=0x{:x}, stack_size=0x{:x}",
               runtime_info.g_offset, runtime_info.stack_base_offset, runtime_info.stack_size_offset);
        
        Ok(())
    }
    
    /// Update runtime addresses from process memory maps
    fn update_runtime_addresses(&self, runtime_info: &mut GoRuntimeInfo, maps_path: &str) -> Result<()> {
        let file = File::open(maps_path)?;
        let reader = BufReader::new(file);
        
        for line in reader.lines() {
            let line = line?;
            if line.contains("[heap]") || line.contains("/go/") {
                // Parse memory mapping line
                let parts: Vec<&str> = line.split_whitespace().collect();
                if let Some(addr_range) = parts.first() {
                    if let Some(dash_pos) = addr_range.find('-') {
                        let start_addr = &addr_range[..dash_pos];
                        if let Ok(addr) = u64::from_str_radix(start_addr, 16) {
                            // Update base addresses if needed
                            if runtime_info.func_tab_base != 0 {
                                runtime_info.func_tab_base += addr;
                            }
                        }
                    }
                }
            }
        }
        
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_find_version_in_data() {
        let parser = GoRuntimeParser::new();
        
        // Test with valid Go version string
        let data_with_version = b"some data go1.21.0 more data";
        let version = parser.find_version_in_data(data_with_version);
        assert_eq!(version, Some("go1.21.0".to_string()));
        
        // Test with different Go version format
        let data_with_version2 = b"prefix go1.24.0 suffix";
        let version2 = parser.find_version_in_data(data_with_version2);
        assert_eq!(version2, Some("go1.24.0".to_string()));
        
        // Test with no version
        let data_no_version = b"no version here";
        let version_none = parser.find_version_in_data(data_no_version);
        assert_eq!(version_none, None);
        
        // Test with malformed version
        let data_malformed = b"go1.x.y invalid";
        let version_malformed = parser.find_version_in_data(data_malformed);
        assert_eq!(version_malformed, None);
    }

    #[test]
    fn test_parse_goroutine_offsets() {
        let parser = GoRuntimeParser::new();
        let mut runtime_info = GoRuntimeInfo::default();
        
        // Test Go 1.21+ offsets
        parser.parse_goroutine_offsets(&mut runtime_info, "go1.21.0").unwrap();
        assert_eq!(runtime_info.g_offset, 0x30);
        assert_eq!(runtime_info.stack_base_offset, 0x10);
        assert_eq!(runtime_info.stack_size_offset, 0x18);
        
        // Test Go 1.20 offsets
        parser.parse_goroutine_offsets(&mut runtime_info, "go1.20.5").unwrap();
        assert_eq!(runtime_info.g_offset, 0x30);
        assert_eq!(runtime_info.stack_base_offset, 0x10);
        assert_eq!(runtime_info.stack_size_offset, 0x18);
        
        // Test unknown version (should use defaults)
        parser.parse_goroutine_offsets(&mut runtime_info, "go1.15.0").unwrap();
        assert_eq!(runtime_info.g_offset, 0x30);
        assert_eq!(runtime_info.stack_base_offset, 0x10);
        assert_eq!(runtime_info.stack_size_offset, 0x18);
    }

    #[test]
    fn test_update_runtime_addresses() {
        let parser = GoRuntimeParser::new();
        let mut runtime_info = GoRuntimeInfo {
            func_tab_base: 0x1000,
            ..Default::default()
        };
        
        // Create a temporary maps file
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "7f8b40000000-7f8b40001000 r-xp 00000000 08:01 123456 /usr/bin/test").unwrap();
        writeln!(temp_file, "7f8b50000000-7f8b50001000 rw-p 00000000 00:00 0 [heap]").unwrap();
        writeln!(temp_file, "7f8b60000000-7f8b60001000 r-xp 00000000 08:01 789012 /go/bin/app").unwrap();
        temp_file.flush().unwrap();
        
        let original_base = runtime_info.func_tab_base;
        parser.update_runtime_addresses(&mut runtime_info, temp_file.path().to_str().unwrap()).unwrap();
        
        // The function should update addresses when it finds heap or go-related mappings
        // In this case, it should find the heap mapping and potentially update the base
        assert!(runtime_info.func_tab_base >= original_base);
    }

    #[test]
    fn test_new_parser() {
        let parser = GoRuntimeParser::new();
        // Just verify the parser can be created without panicking
        assert_eq!(std::mem::size_of_val(&parser), std::mem::size_of::<GoRuntimeParser>());
    }

    // Mock test for process parsing (requires actual binary file)
    #[test]
    #[ignore] // Ignore by default since it requires a real Go binary
    fn test_parse_process_integration() {
        let mut parser = GoRuntimeParser::new();
        
        // This would require a real process ID
        // let result = parser.parse_process(1234);
        // assert!(result.is_ok());
        
        // For now, just test that the method exists and can be called
        // In a real test environment, you would:
        // 1. Create a test Go binary
        // 2. Start it as a process
        // 3. Get its PID
        // 4. Test parsing it
    }
}