use anyhow::{anyhow, Result};
use golang_profile_common::{GoRuntimeInfo, FuncInfo, StackFrame};
use log::{debug, info, warn};
use memmap2::Mmap;
use object::{Object, ObjectSection, ObjectSymbol};
use std::{
    collections::HashMap,
    fs::File,
    io::{Read, Seek, SeekFrom},
};
use byteorder::{LittleEndian, ReadBytesExt};
use procfs::process::Process;

/// Symbol resolver for Go programs
pub struct SymbolResolver {
    pid: u32,
    _runtime_info: GoRuntimeInfo,
    func_table: Vec<FuncInfo>,
    string_table: Vec<u8>,
    symbol_cache: HashMap<u64, String>,
    binary_mmap: Option<Mmap>,
    process_maps: Vec<procfs::process::MemoryMap>,
    base_address: u64,
    kernel_symbols: HashMap<u64, String>,
}

impl SymbolResolver {
    pub fn new(pid: u32, runtime_info: GoRuntimeInfo) -> Result<Self> {
        let mut resolver = SymbolResolver {
            pid,
            _runtime_info: runtime_info,
            func_table: Vec::new(),
            string_table: Vec::new(),
            symbol_cache: HashMap::new(),
            binary_mmap: None,
            process_maps: Vec::new(),
            base_address: 0,
            kernel_symbols: HashMap::new(),
        };
        
        resolver.load_kernel_symbols().unwrap_or_else(|e| {
            warn!("Failed to load kernel symbols: {}", e);
        });
        
        resolver.load_symbols()?;
        Ok(resolver)
    }
    
    /// Load symbols from the target process
    fn load_symbols(&mut self) -> Result<()> {
        let exe_path = format!("/proc/{}/exe", self.pid);
        let file = File::open(&exe_path)
            .map_err(|e| anyhow!("Failed to open executable {}: {}", exe_path, e))?;
        
        let mmap = unsafe { Mmap::map(&file)? };
        let obj = object::File::parse(&*mmap)?;
        
        // Load process memory maps
        self.load_process_maps()?;
        
        // Try fallback to ELF symbols
        self.load_symbols_fallback(&obj)?;
        
        // Original pclntab code (commented out):
        // if let Some(section) = obj.section_by_name(".gopclntab") {
        //     let data = section.data()?;
        //     self.parse_pclntab(data)?;
        // } else {
        //     warn!("No .gopclntab section found, trying alternative methods");
        //     self.load_symbols_fallback(&obj)?;
        // }
        
        self.binary_mmap = Some(mmap);
        info!("Loaded {} function symbols, {} cached symbols", self.func_table.len(), self.symbol_cache.len());
        
        // Debug: print first few symbols and address ranges
        if !self.func_table.is_empty() {
            let min_addr = self.func_table.first().unwrap().entry;
            let max_addr = self.func_table.last().unwrap().entry;
            info!("Symbol address range: 0x{:x} - 0x{:x}", min_addr, max_addr);
            
            for (i, func) in self.func_table.iter().take(5).enumerate() {
                debug!("Function {}: entry=0x{:x}, name_off={}", i, func.entry, func.name_off);
                if let Some(name) = self.get_function_name(func) {
                    debug!("  -> {}", name);
                }
            }
        }
        
        Ok(())
    }
    

    
    /// Fallback symbol loading using standard object symbols
    fn load_symbols_fallback(&mut self, obj: &object::File) -> Result<()> {
        info!("Loading symbols from ELF symbol table");
        
        let mut func_entries = Vec::new();
        
        for symbol in obj.symbols() {
            if let Ok(name) = symbol.name() {
                if symbol.kind() == object::SymbolKind::Text {
                    let address = symbol.address();
                    func_entries.push((address, name.to_string()));
                }
            }
        }
        
        // Sort by address
        func_entries.sort_by_key(|&(addr, _)| addr);
        
        // Try to load Go pclntab for better symbol information
        self.try_load_pclntab(obj)?;
        
        // Convert to FuncInfo and populate cache
        for (addr, name) in func_entries {
            let func_info = FuncInfo {
                entry: addr,
                name_off: 1, // Use 1 to indicate we have a cached name
                file_off: 0,
                line: 0,
            };
            self.func_table.push(func_info);
            self.symbol_cache.insert(addr, name);
        }
        
        info!("Loaded {} functions and {} cached symbols", 
              self.func_table.len(), self.symbol_cache.len());
        
        Ok(())
    }
    
    /// Try to load Go pclntab for enhanced symbol information
    fn try_load_pclntab(&mut self, obj: &object::File) -> Result<()> {
        if let Some(section) = obj.section_by_name(".gopclntab") {
            info!("Found .gopclntab section, parsing Go symbol table");
            let data = section.data()?;
            self.parse_pclntab(data)?;
        } else {
            debug!("No .gopclntab section found");
        }
        Ok(())
    }
    
    /// Parse Go pclntab section for enhanced symbol information
    fn parse_pclntab(&mut self, data: &[u8]) -> Result<()> {
        if data.len() < 8 {
            return Err(anyhow!("pclntab too small"));
        }
        
        // Go pclntab header: magic (4 bytes) + pad (1 byte) + minlc (1 byte) + ptrsize (1 byte) + nfunctab (4 bytes)
        let magic = u32::from_le_bytes([data[0], data[1], data[2], data[3]]);
        // Go has different magic numbers for different versions
        // 0xfffffffb (Go 1.2+), 0xfffffff1 (Go 1.16+), 0xfffffff0 (Go 1.18+)
        if magic != 0xfffffffb && magic != 0xfffffff1 && magic != 0xfffffff0 {
            debug!("Unsupported pclntab magic: 0x{:x}, skipping pclntab parsing", magic);
            return Ok(()); // Don't fail, just skip pclntab parsing
        }
        
        let ptrsize = data[6] as usize;
        if ptrsize != 4 && ptrsize != 8 {
            debug!("Unsupported pointer size: {}, skipping pclntab parsing", ptrsize);
            return Ok(()); // Don't fail, just skip pclntab parsing
        }
        
        // Read function table count
        let nfunctab = if data.len() >= 12 {
            u32::from_le_bytes([data[8], data[9], data[10], data[11]]) as usize
        } else {
            return Err(anyhow!("pclntab header incomplete"));
        };
        
        info!("Found {} functions in pclntab", nfunctab);
        
        // Parse function entries (simplified)
        let mut offset = 8 + ptrsize; // Skip header
        for i in 0..std::cmp::min(nfunctab, 1000) { // Limit to avoid excessive parsing
            if offset + ptrsize * 2 > data.len() {
                break;
            }
            
            let entry = if ptrsize == 8 {
                u64::from_le_bytes([
                    data[offset], data[offset+1], data[offset+2], data[offset+3],
                    data[offset+4], data[offset+5], data[offset+6], data[offset+7]
                ])
            } else {
                u32::from_le_bytes([data[offset], data[offset+1], data[offset+2], data[offset+3]]) as u64
            };
            
            offset += ptrsize;
            
            let funcoff = if ptrsize == 8 {
                u64::from_le_bytes([
                    data[offset], data[offset+1], data[offset+2], data[offset+3],
                    data[offset+4], data[offset+5], data[offset+6], data[offset+7]
                ])
            } else {
                u32::from_le_bytes([data[offset], data[offset+1], data[offset+2], data[offset+3]]) as u64
            };
            
            offset += ptrsize;
            
            // Create enhanced FuncInfo with potential for file/line info
            let func_info = FuncInfo {
                entry,
                name_off: (funcoff & 0xFFFFFFFF) as u32,
                file_off: 0, // Would need more complex parsing
                line: 0,     // Would need more complex parsing
            };
            
            // Only add if not already present
            if !self.func_table.iter().any(|f| f.entry == entry) {
                self.func_table.push(func_info);
            }
        }
        
        // Sort function table by entry address
        self.func_table.sort_by_key(|f| f.entry);
        
        Ok(())
    }
    
    /// Load process memory maps
    fn load_process_maps(&mut self) -> Result<()> {
        let process = Process::new(self.pid as i32)
            .map_err(|e| anyhow!("Failed to open process {}: {}", self.pid, e))?;
        
        let maps = process.maps()
            .map_err(|e| anyhow!("Failed to read process maps: {}", e))?;
        self.process_maps = maps.into_iter().collect();
        
        // Find the base address of the main executable (first executable mapping)
        for map in &self.process_maps {
            if let procfs::process::MMapPath::Path(pathname) = &map.pathname {
                let path_str = pathname.to_string_lossy();
                // Look for the main executable (contains test_program or golang-profile)
                // Check if the mapping is executable (contains 'x' in permissions string)
                let perms_str = format!("{:?}", map.perms);
                if (path_str.contains("test_program") || path_str.contains("golang-profile")) 
                   && perms_str.contains('x') && perms_str.contains('r') {
                    self.base_address = map.address.0;
                    info!("Found base address: 0x{:x} for {}", self.base_address, path_str);
                    break;
                }
            }
        }
        
        if self.base_address == 0 {
            warn!("Could not find base address, using 0x400000 as default");
            self.base_address = 0x400000;
        }
        
        Ok(())
    }
    

    
    /// Resolve a program counter to function name
    pub fn resolve_pc(&self, pc: u64) -> String {
        debug!("Resolving PC: 0x{:x}", pc);
        
        // Check if this is a kernel address (typically >= 0xffffffff80000000 on x86_64)
        if pc >= 0xffffffff80000000 {
            if let Some(kernel_symbol) = self.resolve_kernel_symbol(pc) {
                debug!("Resolved kernel symbol: {}", kernel_symbol);
                return kernel_symbol;
            }
        }
        
        // Check cache first (this contains symbolic library results)
        if let Some(name) = self.symbol_cache.get(&pc) {
            debug!("Found in cache: {}", name);
            return self.clean_go_function_name(name);
        }
        
        // Binary search in function table
        if let Some(func_info) = self.find_function_by_pc(pc) {
            debug!("Found function info: entry=0x{:x}, name_off={}", func_info.entry, func_info.name_off);
            
            // Check if we have a cached name for this function's entry point
            if let Some(cached_name) = self.symbol_cache.get(&func_info.entry) {
                return self.clean_go_function_name(cached_name);
            }
            
            // Try to get complete function information
            if let Some(symbol_info) = self.get_complete_function_info(func_info, pc) {
                debug!("Resolved to complete symbol: {}", symbol_info);
                return symbol_info;
            } else {
                debug!("Failed to get complete function info, trying name only");
                // Fallback to function name only
                if let Some(name) = self.get_function_name(func_info) {
                    debug!("Resolved to function: {}", name);
                    return self.clean_go_function_name(&name);
                }
            }
        } else {
            debug!("No function found for PC 0x{:x}", pc);
        }
        
        // Check nearby cached symbols for better fallback
        for (&addr, symbol) in &self.symbol_cache {
            if addr <= pc && pc < addr + 4096 {
                let cleaned_name = self.clean_go_function_name(symbol);
                return format!("{} +0x{:x}", cleaned_name, pc - addr);
            }
        }
        
        // Fallback to hex address with better formatting
        let addr_str = format!("[unknown:0x{:x}]", pc);
        debug!("Falling back to address: {}", addr_str);
        addr_str
    }
    
    /// Find function info by program counter
    fn find_function_by_pc(&self, pc: u64) -> Option<&FuncInfo> {
        // Binary search for the function containing this PC
        let mut left = 0;
        let mut right = self.func_table.len();
        
        while left < right {
            let mid = (left + right) / 2;
            let func = &self.func_table[mid];
            
            if pc < func.entry {
                right = mid;
            } else if mid + 1 < self.func_table.len() && pc >= self.func_table[mid + 1].entry {
                left = mid + 1;
            } else {
                // Found a candidate function, but check if PC is within reasonable range
                // Assume max function size of 64KB to avoid false matches
                if pc < func.entry + 0x10000 {
                    return Some(func);
                } else {
                    return None;
                }
            }
        }
        
        None
    }
    
    /// Get complete function information including file path and line number
    fn get_complete_function_info(&self, func_info: &FuncInfo, pc: u64) -> Option<String> {
        // Get function name
        let func_name = self.get_function_name(func_info)?;
        
        // Clean up Go function names to be more readable
        let cleaned_name = self.clean_go_function_name(&func_name);
        
        // Get file path
        let file_path = self.get_file_path(func_info);
        
        // Calculate line number based on PC offset
        let line_number = self.get_line_number(func_info, pc);
        
        // Format: funcname /abs/path/file.go:123
        match (file_path, line_number) {
            (Some(file), Some(line)) => {
                Some(format!("{} {}:{}", cleaned_name, file, line))
            }
            (Some(file), None) => {
                Some(format!("{} {}", cleaned_name, file))
            }
            _ => Some(cleaned_name)
        }
    }
    
    /// Clean up Go function names for better readability
    fn clean_go_function_name(&self, name: &str) -> String {
        // Remove common Go runtime prefixes and make names more readable
        let cleaned = name
            .replace("runtime.", "")
            .replace("main.", "main::")
            .replace(".func", "::func")
            .replace("·", "::")
            .replace("/", "::")
            .replace("(*", "(")
            .replace(")", ")");
            
        // If it's still a hex address, try to make it more meaningful
        if cleaned.starts_with("0x") {
            format!("[unknown:{}]", &cleaned[2..std::cmp::min(cleaned.len(), 10)])
        } else {
            cleaned
        }
    }
    
    /// Get function name from function info
    fn get_function_name(&self, func_info: &FuncInfo) -> Option<String> {
        // If name_off is 1, it means we have a cached name (fallback mode)
        if func_info.name_off == 1 {
            if let Some(name) = self.symbol_cache.get(&func_info.entry) {
                return Some(name.clone());
            }
        }
        
        // name_off can be 0 if it points to the first string in the table
        
        let name_offset = func_info.name_off as usize;
        debug!("String table size: {}, name_offset: {}", self.string_table.len(), name_offset);
        if name_offset >= self.string_table.len() {
            debug!("name_offset {} >= string_table.len() {}", name_offset, self.string_table.len());
            return None;
        }
        
        // Find null terminator
        let mut end = name_offset;
        while end < self.string_table.len() && self.string_table[end] != 0 {
            end += 1;
        }
        
        if end > name_offset {
            String::from_utf8(self.string_table[name_offset..end].to_vec()).ok()
        } else {
            None
        }
    }
    
    /// Get file path from function info
    fn get_file_path(&self, func_info: &FuncInfo) -> Option<String> {
        if func_info.file_off == 0 {
            return None;
        }
        
        let file_offset = func_info.file_off as usize;
        if file_offset >= self.string_table.len() {
            return None;
        }
        
        // Find null terminator
        let mut end = file_offset;
        while end < self.string_table.len() && self.string_table[end] != 0 {
            end += 1;
        }
        
        if end > file_offset {
            String::from_utf8(self.string_table[file_offset..end].to_vec()).ok()
        } else {
            None
        }
    }
    
    /// Get line number from function info and PC
    fn get_line_number(&self, func_info: &FuncInfo, _pc: u64) -> Option<u32> {
        // For now, return the base line number from func_info
        // In a more complete implementation, we would parse the line table
        // to get the exact line number for the given PC
        if func_info.line > 0 {
            Some(func_info.line)
        } else {
            None
        }
    }
    
    /// Load kernel symbols from /proc/kallsyms
    fn load_kernel_symbols(&mut self) -> Result<()> {
        let kallsyms_path = "/proc/kallsyms";
        let content = std::fs::read_to_string(kallsyms_path)
            .map_err(|e| anyhow!("Failed to read {}: {}", kallsyms_path, e))?;
        
        for line in content.lines() {
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() >= 3 {
                if let Ok(addr) = u64::from_str_radix(parts[0], 16) {
                    let symbol_type = parts[1];
                    let symbol_name = parts[2];
                    
                    // Only include function symbols (T, t, W, w)
                    if matches!(symbol_type, "T" | "t" | "W" | "w") {
                        self.kernel_symbols.insert(addr, symbol_name.to_string());
                    }
                }
            }
        }
        
        info!("Loaded {} kernel symbols", self.kernel_symbols.len());
        Ok(())
    }
    
    /// Resolve kernel symbol by address
    pub fn resolve_kernel_symbol(&self, addr: u64) -> Option<String> {
        // First try exact match
        if let Some(symbol) = self.kernel_symbols.get(&addr) {
            return Some(symbol.clone());
        }
        
        // Find the closest symbol before this address
        let mut best_addr = 0;
        let mut best_symbol = None;
        
        for (&sym_addr, symbol) in &self.kernel_symbols {
            if sym_addr <= addr && sym_addr > best_addr {
                best_addr = sym_addr;
                best_symbol = Some(symbol);
            }
        }
        
        if let Some(symbol) = best_symbol {
            if addr - best_addr < 0x10000 { // Within 64KB range
                if addr == best_addr {
                    Some(symbol.clone())
                } else {
                    Some(format!("{} +0x{:x}", symbol, addr - best_addr))
                }
            } else {
                None
            }
        } else {
            None
        }
    }

}

#[cfg(test)]
mod tests {
    use super::*;
    use golang_profile_common::{StackFrame, FuncInfo};
    use std::io::{Cursor, Write};
    use byteorder::{LittleEndian, WriteBytesExt};

    fn create_mock_runtime_info() -> GoRuntimeInfo {
        GoRuntimeInfo {
            version: *b"go1.21.0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0\0",
            g_offset: 0x30,
            stack_base_offset: 0x10,
            stack_size_offset: 0x18,
            func_tab_base: 0x400000,
            func_tab_size: 1000,
            pc_quantum: 1,
        }
    }

    fn create_mock_pclntab_data() -> Vec<u8> {
        let mut data = Vec::new();
        let mut cursor = Cursor::new(&mut data);
        
        // Write pclntab header (64 bytes total)
        cursor.write_u32::<LittleEndian>(0xFFFFFFF1).unwrap(); // magic for Go 1.20+
        cursor.write_u8(0).unwrap(); // pad1
        cursor.write_u8(0).unwrap(); // pad2
        cursor.write_u8(1).unwrap(); // min_lc
        cursor.write_u8(8).unwrap(); // ptr_size
        
        cursor.write_u64::<LittleEndian>(2).unwrap(); // nfunc
        cursor.write_u64::<LittleEndian>(1).unwrap(); // nfiles
        cursor.write_u64::<LittleEndian>(0x401000).unwrap(); // text_start
        cursor.write_u64::<LittleEndian>(64).unwrap(); // functab offset (function table starts at 64)
        cursor.write_u64::<LittleEndian>(200).unwrap(); // func_name_tab offset
        cursor.write_u64::<LittleEndian>(0).unwrap(); // cutab
        cursor.write_u64::<LittleEndian>(80).unwrap(); // funcdata offset (function data starts at 80)
        cursor.write_u64::<LittleEndian>(0).unwrap(); // pctab
        
        // Write functab entries (starting at offset 64)
        // Each entry is 8 bytes: pc_offset (4 bytes) + funcdata_offset (4 bytes)
        cursor.set_position(64);
        
        // Function 1 functab entry
        cursor.write_u32::<LittleEndian>(0x100).unwrap(); // pc_offset from text_start
        cursor.write_u32::<LittleEndian>(80).unwrap(); // funcdata_offset (points to function data)
        
        // Function 2 functab entry
        cursor.write_u32::<LittleEndian>(0x200).unwrap(); // pc_offset from text_start
        cursor.write_u32::<LittleEndian>(120).unwrap(); // funcdata_offset (points to second function data)
        
        // Write funcdata entries (starting at offset 80)
        cursor.set_position(80);
        
        // Function 1 data (40 bytes)
        cursor.write_u64::<LittleEndian>(0x401100).unwrap(); // entry (absolute address)
        cursor.write_u32::<LittleEndian>(0).unwrap(); // name_off (points to string table)
        cursor.write_u32::<LittleEndian>(0).unwrap(); // args
        cursor.write_u32::<LittleEndian>(0).unwrap(); // deferreturn
        cursor.write_u32::<LittleEndian>(0).unwrap(); // pcsp
        cursor.write_u32::<LittleEndian>(0).unwrap(); // pcfile
        cursor.write_u32::<LittleEndian>(0).unwrap(); // pcln
        cursor.write_u32::<LittleEndian>(0).unwrap(); // npcdata
        cursor.write_u32::<LittleEndian>(0).unwrap(); // cuoff
        cursor.write_u8(0).unwrap(); // funcid
        cursor.write_u8(0).unwrap(); // flag
        cursor.write_u16::<LittleEndian>(0).unwrap(); // pad
        cursor.write_u8(0).unwrap(); // nfuncdata
        
        // Function 2 data (40 bytes)
        cursor.set_position(120);
        cursor.write_u64::<LittleEndian>(0x401200).unwrap(); // entry (absolute address)
        cursor.write_u32::<LittleEndian>(5).unwrap(); // name_off (points to second string)
        cursor.write_u32::<LittleEndian>(0).unwrap(); // args
        cursor.write_u32::<LittleEndian>(0).unwrap(); // deferreturn
        cursor.write_u32::<LittleEndian>(0).unwrap(); // pcsp
        cursor.write_u32::<LittleEndian>(0).unwrap(); // pcfile
        cursor.write_u32::<LittleEndian>(0).unwrap(); // pcln
        cursor.write_u32::<LittleEndian>(0).unwrap(); // npcdata
        cursor.write_u32::<LittleEndian>(0).unwrap(); // cuoff
        cursor.write_u8(0).unwrap(); // funcid
        cursor.write_u8(0).unwrap(); // flag
        cursor.write_u16::<LittleEndian>(0).unwrap(); // pad
        cursor.write_u8(0).unwrap(); // nfuncdata
        
        // Write string table at offset 200
        cursor.set_position(200);
        cursor.write_all(b"main\0test\0").unwrap();
        
        data
    }



    #[test]
    fn test_get_function_name() {
        let runtime_info = create_mock_runtime_info();
        let mut resolver = SymbolResolver {
            pid: 1234,
            _runtime_info: runtime_info,
            func_table: Vec::new(),
            string_table: b"main\0test\0".to_vec(),
            symbol_cache: HashMap::new(),
            binary_mmap: None,
            process_maps: Vec::new(),
            base_address: 0,
            kernel_symbols: HashMap::new(),
        };
        
        let func_info1 = FuncInfo {
            entry: 0x401100,
            name_off: 0,
            file_off: 0,
            line: 0,
        };
        
        let func_info2 = FuncInfo {
            entry: 0x401200,
            name_off: 5,
            file_off: 0,
            line: 0,
        };
        
        let name1 = resolver.get_function_name(&func_info1);
        assert_eq!(name1, Some("main".to_string()));
        
        let name2 = resolver.get_function_name(&func_info2);
        assert_eq!(name2, Some("test".to_string()));
        
        // Test invalid offset
        let func_info_invalid = FuncInfo {
            entry: 0x401300,
            name_off: 100, // Beyond string table
            file_off: 0,
            line: 0,
        };
        
        let name_invalid = resolver.get_function_name(&func_info_invalid);
        assert_eq!(name_invalid, None);
    }

    #[test]
    fn test_find_function_by_pc() {
        let runtime_info = create_mock_runtime_info();
        let mut resolver = SymbolResolver {
            pid: 1234,
            _runtime_info: runtime_info,
            func_table: vec![
                FuncInfo { entry: 0x401000, name_off: 0, file_off: 0, line: 0 },
                FuncInfo { entry: 0x401100, name_off: 5, file_off: 0, line: 0 },
                FuncInfo { entry: 0x401200, name_off: 10, file_off: 0, line: 0 },
            ],
            string_table: Vec::new(),
            symbol_cache: HashMap::new(),
            binary_mmap: None,
            process_maps: Vec::new(),
            base_address: 0,
            kernel_symbols: HashMap::new(),
        };
        
        // Test exact match
        let func = resolver.find_function_by_pc(0x401100);
        assert!(func.is_some());
        assert_eq!(func.unwrap().entry, 0x401100);
        
        // Test PC within function range
        let func = resolver.find_function_by_pc(0x401150);
        assert!(func.is_some());
        assert_eq!(func.unwrap().entry, 0x401100);
        
        // Test PC before first function
        let func = resolver.find_function_by_pc(0x400500);
        assert!(func.is_none());
        
        // Test PC after last function
        let func = resolver.find_function_by_pc(0x401300);
        assert!(func.is_some());
        assert_eq!(func.unwrap().entry, 0x401200);
    }

    #[test]
    fn test_resolve_pc() {
        let runtime_info = create_mock_runtime_info();
        let mut resolver = SymbolResolver {
            pid: 1234,
            _runtime_info: runtime_info,
            func_table: vec![
                FuncInfo { entry: 0x401100, name_off: 0, file_off: 0, line: 0 },
            ],
            string_table: b"main\0".to_vec(),
            symbol_cache: HashMap::new(),
            binary_mmap: None,
            process_maps: Vec::new(),
            base_address: 0,
            kernel_symbols: HashMap::new(),
        };
        
        // Test successful resolution
        let name = resolver.resolve_pc(0x401100);
        assert_eq!(name, "main");
        
        // Test fallback to hex address
        let name = resolver.resolve_pc(0x500000);
        assert_eq!(name, "0x500000");
    }






}