// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// This implements Go 1.2+ .pclntab symbol parsing as defined
// in http://golang.org/s/go12symtab. The Golang runtime implementation of
// this is in go/src/runtime/symtab.go, but unfortunately it is not exported.

use std::collections::HashMap;
use std::mem;
use object::{Object, ObjectSection};
use log::{debug, warn};

// Go runtime functions for which we should not attempt to unwind further
pub fn get_go_functions_stop_delta() -> HashMap<String, bool> {
    let mut map = HashMap::new();
    map.insert("runtime.mstart".to_string(), true); // topmost for the go runtime main stacks
    map.insert("runtime.goexit".to_string(), true); // return address in all goroutine stacks
    
    // stack switch functions that would need special handling for further unwinding.
    map.insert("runtime.mcall".to_string(), true);
    map.insert("runtime.systemstack".to_string(), true);
    
    // signal return frame
    map.insert("runtime.sigreturn".to_string(), true);
    map.insert("runtime.sigreturn__sigaction".to_string(), true);
    
    map
}

const MAX_BYTES_GO_PCLNTAB: usize = 128 * 1024 * 1024;

// internally used gopclntab version
const GO_INVALID: u8 = 0;
const GO1_2: u8 = 2;
const GO1_16: u8 = 16;
const GO1_18: u8 = 18;
const GO1_20: u8 = 20;

pub fn go_magic_to_version(magic: u32) -> u8 {
    match magic {
        0xfffffffb => GO1_2,   // Go 1.2
        0xfffffffa => GO1_16,  // Go 1.16
        0xfffffff0 => GO1_18,  // Go 1.18
        0xfffffff1 => GO1_20,  // Go 1.20
        _ => GO_INVALID,
    }
}

// pclntabHeader is the Golang pclntab header structure
#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct PclntabHeader {
    pub magic: u32,      // magic is one of the magicGo1_xx constants identifying the version
    pub pad: u16,        // pad is unused and is needed for alignment
    pub quantum: u8,     // quantum is the CPU instruction size alignment (e.g. 1 for x86, 4 for arm)
    pub ptr_size: u8,    // ptr_size is the CPU pointer size in bytes
    pub num_funcs: u64,  // num_funcs is the number of function definitions to follow
}

// pclntabHeader116 is the Golang pclntab header structure starting Go 1.16
#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct PclntabHeader116 {
    pub header: PclntabHeader,
    pub nfiles: usize,
    pub funcname_offset: usize,
    pub cu_offset: usize,
    pub filetab_offset: usize,
    pub pctab_offset: usize,
    pub pcln_offset: usize,
}

// pclntabHeader118 is the Golang pclntab header structure starting Go 1.18
#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct PclntabHeader118 {
    pub header: PclntabHeader,
    pub nfiles: usize,
    pub text_start: usize,
    pub funcname_offset: usize,
    pub cu_offset: usize,
    pub filetab_offset: usize,
    pub pctab_offset: usize,
    pub pcln_offset: usize,
}

// pclntabFuncMap is the Golang function symbol table map entry
#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct PclntabFuncMap {
    pub pc: usize,
    pub func_off: usize,
}

// pclntabFuncMap118 is the Golang function symbol table map entry for Go 1.18+.
#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct PclntabFuncMap118 {
    pub pc: u32,
    pub func_off: u32,
}

// pclntabFunc is the common portion of the Golang function definition.
#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct PclntabFunc {
    pub name_off: i32,
    pub args_size: i32,
    pub frame_size: i32,
    pub pcsp_off: i32,
    pub pcfile_off: i32,
    pub pcln_off: i32,
    pub nfunc_data: i32,
    pub npc_data: i32,
}

// pcval describes a Program Counter (pc) and a value (val) associated with it
pub struct Pcval {
    pub ptr: Vec<u8>,
    pub pc_start: u64,
    pub pc_end: u64,
    pub val: i32,
    pub quantum: u8,
}

impl Pcval {
    pub fn new(data: Vec<u8>, pc: u64, quantum: u8) -> Self {
        let mut p = Pcval {
            ptr: data,
            pc_start: 0,
            pc_end: pc,
            val: -1,
            quantum,
        };
        p.step();
        p
    }
    
    // getInt reads one zig-zag encoded integer
    pub fn get_int(&mut self) -> u32 {
        let mut v = 0u32;
        let mut shift = 0u32;
        
        loop {
            if self.ptr.is_empty() {
                return 0;
            }
            let b = self.ptr.remove(0);
            v |= ((b & 0x7F) as u32) << shift;
            if b & 0x80 == 0 {
                break;
            }
            shift += 7;
        }
        v
    }
    
    // step executes one line of the pcval table. Returns true on success.
    pub fn step(&mut self) -> bool {
        if self.ptr.is_empty() || self.ptr[0] == 0 {
            return false;
        }
        
        self.pc_start = self.pc_end;
        let mut d = self.get_int();
        
        if d & 1 != 0 {
            d = !(d >> 1);
        } else {
            d >>= 1;
        }
        
        self.val += d as i32;
        self.pc_end += (self.get_int() * self.quantum as u32) as u64;
        true
    }
}

// Gopclntab is the API for extracting data from .gopclntab
pub struct Gopclntab {
    pub data: Vec<u8>,
    pub text_start: usize,
    pub num_funcs: usize,
    
    pub version: u8,
    pub quantum: u8,
    pub ptr_size: u8,
    pub fun_size: u8,
    pub func_map_size: u8,
    
    // These are byte slices to various areas within .gopclntab
    pub functab: Vec<u8>,
    pub funcdata: Vec<u8>,
    pub funcnametab: Vec<u8>,
    pub filetab: Vec<u8>,
    pub pctab: Vec<u8>,
    pub cutab: Vec<u8>,
}

impl Gopclntab {
    pub fn new(data: Vec<u8>) -> Result<Self, String> {
        let hdr_size = mem::size_of::<PclntabHeader>();
        let data_len = data.len();
        
        if data_len < hdr_size {
            return Err(format!(".gopclntab is too short ({})", data_len));
        }
        
        // Safety: We've checked the bounds above
        let hdr = unsafe {
            &*(data.as_ptr() as *const PclntabHeader)
        };
        
        let version = go_magic_to_version(hdr.magic);
        if version == GO_INVALID || hdr.pad != 0 || hdr.ptr_size != 8 {
            let magic = hdr.magic;
            let pad = hdr.pad;
            let ptr_size = hdr.ptr_size;
            return Err(format!(".gopclntab header: {:x}, {:x}, {:x}", magic, pad, ptr_size));
        }
        
        let mut gopclntab = Gopclntab {
            data: data.clone(),
            text_start: 0,
            num_funcs: hdr.num_funcs as usize,
            version,
            quantum: hdr.quantum,
            ptr_size: hdr.ptr_size,
            fun_size: hdr.ptr_size + mem::size_of::<PclntabFunc>() as u8,
            func_map_size: hdr.ptr_size * 2,
            functab: Vec::new(),
            funcdata: Vec::new(),
            funcnametab: Vec::new(),
            filetab: Vec::new(),
            pctab: Vec::new(),
            cutab: Vec::new(),
        };
        
        match version {
            GO1_2 => {
                let functab_end = hdr_size + gopclntab.num_funcs * gopclntab.func_map_size as usize + hdr.ptr_size as usize;
                let filetab_offset = get_int32(&data, functab_end)
                    .ok_or("Failed to read filetab_offset")?;
                let num_source_files = get_int32(&data, filetab_offset as usize)
                    .ok_or("Failed to read num_source_files")?;
                
                if filetab_offset == 0 || num_source_files == 0 {
                    return Err(format!(".gopclntab corrupt (filetab 0x{:x}, nfiles {})", filetab_offset, num_source_files));
                }
                
                gopclntab.functab = data[hdr_size..filetab_offset as usize].to_vec();
                gopclntab.cutab = data[filetab_offset as usize..].to_vec();
                gopclntab.pctab = data.clone();
                gopclntab.funcnametab = data.clone();
                gopclntab.funcdata = data.clone();
                gopclntab.filetab = data.clone();
            },
            GO1_16 => {
                let hdr_size = mem::size_of::<PclntabHeader116>();
                if data_len < hdr_size {
                    return Err(format!(".gopclntab is too short ({})", data_len));
                }
                
                let hdr116 = unsafe {
                    &*(data.as_ptr() as *const PclntabHeader116)
                };
                
                let funcname_offset = hdr116.funcname_offset;
                let cu_offset = hdr116.cu_offset;
                let filetab_offset = hdr116.filetab_offset;
                let pctab_offset = hdr116.pctab_offset;
                let pcln_offset = hdr116.pcln_offset;
                
                if data_len < funcname_offset || data_len < cu_offset ||
                   data_len < filetab_offset || data_len < pctab_offset ||
                   data_len < pcln_offset {
                    return Err(format!(".gopclntab is corrupt ({:x}, {:x}, {:x}, {:x}, {:x})",
                        funcname_offset, cu_offset,
                        filetab_offset, pctab_offset,
                        pcln_offset));
                }
                
                gopclntab.funcnametab = data[funcname_offset..].to_vec();
                gopclntab.cutab = data[cu_offset..].to_vec();
                gopclntab.filetab = data[filetab_offset..].to_vec();
                gopclntab.pctab = data[pctab_offset..].to_vec();
                gopclntab.functab = data[pcln_offset..].to_vec();
                gopclntab.funcdata = gopclntab.functab.clone();
            },
            GO1_18 | GO1_20 => {
                let hdr_size = mem::size_of::<PclntabHeader118>();
                if data_len < hdr_size {
                    return Err(format!(".gopclntab is too short ({})", data_len));
                }
                
                let hdr118 = unsafe {
                    &*(data.as_ptr() as *const PclntabHeader118)
                };
                
                let funcname_offset = hdr118.funcname_offset;
                let cu_offset = hdr118.cu_offset;
                let filetab_offset = hdr118.filetab_offset;
                let pctab_offset = hdr118.pctab_offset;
                let pcln_offset = hdr118.pcln_offset;
                let text_start = hdr118.text_start;
                
                if data_len < funcname_offset || data_len < cu_offset ||
                   data_len < filetab_offset || data_len < pctab_offset ||
                   data_len < pcln_offset {
                    return Err(format!(".gopclntab is corrupt ({:x}, {:x}, {:x}, {:x}, {:x})",
                        funcname_offset, cu_offset,
                        filetab_offset, pctab_offset,
                        pcln_offset));
                }
                
                gopclntab.funcnametab = data[funcname_offset..].to_vec();
                gopclntab.cutab = data[cu_offset..].to_vec();
                gopclntab.filetab = data[filetab_offset..].to_vec();
                gopclntab.pctab = data[pctab_offset..].to_vec();
                gopclntab.functab = data[pcln_offset..].to_vec();
                gopclntab.funcdata = gopclntab.functab.clone();
                gopclntab.text_start = text_start;
                
                // With the change of the type of the first field of _func in Go 1.18
                gopclntab.func_map_size = 2 * 4;
                gopclntab.fun_size = 4 + mem::size_of::<PclntabFunc>() as u8;
            },
            _ => {
                return Err(format!("Unsupported Go version: {}", version));
            }
        }
        
        Ok(gopclntab)
    }
    
    // getFuncMapEntry returns the entry at 'index' from the gopclntab function lookup map.
    pub fn get_func_map_entry(&self, index: usize) -> (usize, usize) {
        if self.version >= GO1_18 {
            let offset = index * self.func_map_size as usize;
            if offset + 8 <= self.functab.len() {
                let fmap = unsafe {
                    &*(self.functab[offset..].as_ptr() as *const PclntabFuncMap118)
                };
                return (self.text_start + fmap.pc as usize, fmap.func_off as usize);
            }
        } else {
            let offset = index * self.func_map_size as usize;
            if offset + 16 <= self.functab.len() {
                let fmap = unsafe {
                    &*(self.functab[offset..].as_ptr() as *const PclntabFuncMap)
                };
                return (fmap.pc, fmap.func_off);
            }
        }
        (0, 0)
    }
    
    // getFunc returns the gopclntab function data and its start address.
    pub fn get_func(&self, mut func_off: usize) -> Option<(usize, PclntabFunc)> {
        if self.funcdata.len() < func_off + self.fun_size as usize {
            return None;
        }
        
        let pc = if self.version >= GO1_18 {
            if func_off + 4 > self.funcdata.len() {
                return None;
            }
            let pc_offset = u32::from_le_bytes([
                self.funcdata[func_off],
                self.funcdata[func_off + 1],
                self.funcdata[func_off + 2],
                self.funcdata[func_off + 3],
            ]);
            func_off += 4;
            self.text_start + pc_offset as usize
        } else {
            if func_off + 8 > self.funcdata.len() {
                return None;
            }
            let pc = usize::from_le_bytes([
                self.funcdata[func_off],
                self.funcdata[func_off + 1],
                self.funcdata[func_off + 2],
                self.funcdata[func_off + 3],
                self.funcdata[func_off + 4],
                self.funcdata[func_off + 5],
                self.funcdata[func_off + 6],
                self.funcdata[func_off + 7],
            ]);
            func_off += self.ptr_size as usize;
            pc
        };
        
        if func_off + mem::size_of::<PclntabFunc>() > self.funcdata.len() {
            return None;
        }
        
        let func = unsafe {
            *(self.funcdata[func_off..].as_ptr() as *const PclntabFunc)
        };
        
        Some((pc, func))
    }
    
    // getPcval returns the pcval table at given offset with 'startPc' as the pc start value.
    pub fn get_pcval(&self, offs: i32, start_pc: u64) -> Option<Pcval> {
        let offset = offs as usize;
        if offset < self.pctab.len() {
            Some(Pcval::new(self.pctab[offset..].to_vec(), start_pc, self.quantum))
        } else {
            None
        }
    }
    
    // mapPcval steps the given pcval table until matching PC is found and returns the value.
    pub fn map_pcval(&self, offs: i32, start_pc: u64, pc: u64) -> Option<i32> {
        let mut p = self.get_pcval(offs, start_pc)?;
        
        while pc >= p.pc_end {
            if !p.step() {
                return None;
            }
        }
        
        Some(p.val)
    }
    
    // Symbolize returns the file, line and function information for given PC
    pub fn symbolize(&self, pc: usize) -> (String, u32, String) {
        // Binary search to find the function containing this PC
        let mut left = 0;
        let mut right = self.num_funcs;
        
        while left < right {
            let mid = (left + right) / 2;
            let (func_pc, _) = self.get_func_map_entry(mid);
            
            if func_pc > pc {
                right = mid;
            } else {
                left = mid + 1;
            }
        }
        
        if left == 0 {
            return (String::new(), 0, String::new());
        }
        
        let index = left - 1;
        let (map_pc, func_off) = self.get_func_map_entry(index);
        
        if let Some((func_pc, func)) = self.get_func(func_off) {
            if map_pc != func_pc {
                return (String::new(), 0, String::new());
            }
            
            let func_name = get_string(&self.funcnametab, func.name_off as usize);
            
            let mut source_file = String::new();
            let mut line = 0u32;
            
            if func.pcfile_off != 0 {
                if let Some(file_index) = self.map_pcval(func.pcfile_off, func_pc as u64, pc as u64) {
                    let mut file_index = file_index;
                    if self.version >= GO1_16 {
                        file_index += func.npc_data;
                    }
                    
                    if let Some(file_offset) = get_int32(&self.cutab, 4 * file_index as usize) {
                        source_file = get_string(&self.filetab, file_offset as usize);
                    }
                }
            }
            
            if func.pcln_off != 0 {
                if let Some(line_no) = self.map_pcval(func.pcln_off, func_pc as u64, pc as u64) {
                    line = line_no as u32;
                }
            }
            
            return (source_file, line, func_name);
        }
        
        (String::new(), 0, String::new())
    }
}

// Helper functions

// getInt32 gets a 32-bit integer from the data slice at offset with bounds checking
pub fn get_int32(data: &[u8], offset: usize) -> Option<i32> {
    if offset + 4 <= data.len() {
        Some(i32::from_le_bytes([
            data[offset],
            data[offset + 1],
            data[offset + 2],
            data[offset + 3],
        ]))
    } else {
        None
    }
}

// getString returns a string from the data slice at given offset.
pub fn get_string(data: &[u8], offset: usize) -> String {
    if offset >= data.len() {
        return String::new();
    }
    
    let slice = &data[offset..];
    if let Some(zero_idx) = slice.iter().position(|&b| b == 0) {
        String::from_utf8_lossy(&slice[..zero_idx]).to_string()
    } else {
        String::new()
    }
}

// pclntabHeaderSignature returns a byte slice that can be
// used to verify if some bytes represent a valid pclntab header.
pub fn pclntab_header_signature(arch: &str) -> Vec<u8> {
    let quantum = match arch {
        "x86_64" => 0x1,
        "aarch64" => 0x4,
        _ => 0x1, // default to x86_64
    };
    
    // - the first byte is ignored and not included in this signature
    //   as it is different per Go version (see magicGo1_XX)
    // - next three bytes are 0xff (shared on magicGo1_XX)
    // - pad is zero (two bytes)
    // - quantum depends on the architecture
    // - ptrSize is 8 for 64 bit systems (arm64 and amd64)
    vec![0xff, 0xff, 0xff, 0x00, 0x00, quantum, 0x08]
}

// searchGoPclntab uses heuristic to find the gopclntab from RO data.
pub fn search_go_pclntab(data: &[u8], arch: &str) -> Option<Vec<u8>> {
    let signature = pclntab_header_signature(arch);
    let hdr_size = mem::size_of::<PclntabHeader>();
    
    if data.len() < hdr_size {
        return None;
    }
    
    let mut i = 1;
    while i < data.len() - hdr_size {
        // Search for something looking like a valid pclntabHeader header
        // Ignore the first byte on search (differs on magicGo1_XXX)
        if let Some(n) = data[i..].windows(signature.len()).position(|window| window == signature) {
            i += n;
            
            // Check the 'magic' against supported list
            if i >= 1 {
                let magic_offset = i - 1;
                if magic_offset + 4 <= data.len() {
                    let magic = u32::from_le_bytes([
                        data[magic_offset],
                        data[magic_offset + 1],
                        data[magic_offset + 2],
                        data[magic_offset + 3],
                    ]);
                    
                    if go_magic_to_version(magic) != GO_INVALID {
                        return Some(data[magic_offset..].to_vec());
                    }
                }
            }
        } else {
            break;
        }
        i += 8;
    }
    
    None
}