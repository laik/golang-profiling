use anyhow::{anyhow, Result};
use gimli::{Dwarf, EndianSlice, LittleEndian, Reader};
use object::{Object, ObjectSection};
use std::collections::HashMap;
use std::path::Path;

/// DWARF 调试信息解析器
pub struct DwarfParser {
    /// 函数地址到源码位置的映射
    pub address_to_location: HashMap<u64, SourceLocation>,
    /// 函数名到地址范围的映射
    pub function_ranges: HashMap<String, Vec<(u64, u64)>>,
}

/// 源码位置信息
#[derive(Debug, Clone)]
pub struct SourceLocation {
    pub file_path: String,
    pub line: u32,
    pub column: u32,
    pub function_name: Option<String>,
}

impl DwarfParser {
    /// 创建新的 DWARF 解析器
    pub fn new() -> Self {
        Self {
            address_to_location: HashMap::new(),
            function_ranges: HashMap::new(),
        }
    }

    /// 从二进制文件解析 DWARF 调试信息
    pub fn parse_from_file<P: AsRef<Path>>(&mut self, file_path: P) -> Result<()> {
        let file_data = std::fs::read(file_path)?;
        let object = object::File::parse(&file_data[..])?;
        self.parse_from_object(&object)
    }

    /// 从内存中的二进制数据解析 DWARF 调试信息
    pub fn parse_from_memory(&mut self, data: &[u8]) -> Result<()> {
        let object = object::File::parse(data)?;
        self.parse_from_object(&object)
    }

    /// 从 object 文件解析 DWARF 调试信息
    fn parse_from_object(&mut self, object: &object::File) -> Result<()> {
        // Store section data to avoid lifetime issues
        let mut section_data: HashMap<String, Vec<u8>> = HashMap::new();
        
        // Load all DWARF sections
        for section_id in [
            gimli::SectionId::DebugInfo,
            gimli::SectionId::DebugAbbrev,
            gimli::SectionId::DebugLine,
            gimli::SectionId::DebugStr,
            gimli::SectionId::DebugRanges,
            gimli::SectionId::DebugLoc,
        ] {
            if let Some(section) = object.section_by_name(section_id.name()) {
                if let Ok(data) = section.uncompressed_data() {
                    section_data.insert(section_id.name().to_string(), data.into_owned());
                }
            }
        }
        
        let load_section = |id: gimli::SectionId| -> Result<EndianSlice<LittleEndian>, gimli::Error> {
            match section_data.get(id.name()) {
                Some(data) => Ok(EndianSlice::new(data, LittleEndian)),
                None => Ok(EndianSlice::new(&[], LittleEndian)),
            }
        };

        let dwarf = Dwarf::load(&load_section)?;

        // 遍历编译单元
        let mut iter = dwarf.units();
        while let Some(header) = iter.next()? {
            let unit = dwarf.unit(header)?;
            self.parse_compilation_unit(&dwarf, &unit)?;
        }

        Ok(())
    }

    /// 解析编译单元
    fn parse_compilation_unit<R: Reader>(
        &mut self,
        dwarf: &Dwarf<R>,
        unit: &gimli::Unit<R>,
    ) -> Result<()> {
        // 获取行号程序
        if let Some(line_program) = unit.line_program.clone() {
            self.parse_line_program(dwarf, unit, line_program)?;
        }

        // 解析 DIE (Debug Information Entry)
        let mut entries = unit.entries();
        while let Some((_, entry)) = entries.next_dfs()? {
            if entry.tag() == gimli::DW_TAG_subprogram {
                self.parse_function_die(dwarf, unit, entry)?;
            }
        }

        Ok(())
    }

    /// 解析行号程序，建立地址到源码位置的映射
    fn parse_line_program<R: Reader>(
        &mut self,
        dwarf: &Dwarf<R>,
        unit: &gimli::Unit<R>,
        line_program: gimli::IncompleteLineProgram<R>,
    ) -> Result<()> {
        let (program, sequences) = line_program.sequences()?;
        
        for sequence in sequences {
            let mut rows = program.resume_from(&sequence);
            while let Some((header, row)) = rows.next_row()? {
                if let Some(file) = row.file(header) {
                    let file_path = self.get_file_path(dwarf, unit, file)?;
                    
                    let location = SourceLocation {
                        file_path,
                        line: row.line().map(|l| l.get() as u32).unwrap_or(0),
                        column: match row.column() {
                            gimli::ColumnType::LeftEdge => 0,
                            gimli::ColumnType::Column(c) => c.get() as u32,
                        },
                        function_name: None, // 将在后续步骤中填充
                    };
                    
                    self.address_to_location.insert(row.address(), location);
                }
            }
        }
        
        Ok(())
    }

    /// 解析函数 DIE，获取函数信息
    fn parse_function_die<R: Reader>(
        &mut self,
        dwarf: &Dwarf<R>,
        unit: &gimli::Unit<R>,
        entry: &gimli::DebuggingInformationEntry<R>,
    ) -> Result<()> {
        let mut function_name = None;
        let mut low_pc = None;
        let mut high_pc = None;
        let mut ranges = None;

        // 解析函数属性
        let mut attrs = entry.attrs();
        while let Some(attr) = attrs.next()? {
            match attr.name() {
                gimli::DW_AT_name => {
                    if let Ok(name) = dwarf.attr_string(unit, attr.value()) {
                        function_name = Some(name.to_string_lossy()?.into_owned());
                    }
                }
                gimli::DW_AT_low_pc => {
                    if let gimli::AttributeValue::Addr(addr) = attr.value() {
                        low_pc = Some(addr);
                    }
                }
                gimli::DW_AT_high_pc => {
                    match attr.value() {
                        gimli::AttributeValue::Addr(addr) => high_pc = Some(addr),
                        gimli::AttributeValue::Udata(offset) => {
                            if let Some(low) = low_pc {
                                high_pc = Some(low + offset);
                            }
                        }
                        _ => {}
                    }
                }
                gimli::DW_AT_ranges => {
                    if let gimli::AttributeValue::RangeListsRef(offset) = attr.value() {
                        ranges = Some(offset);
                    }
                }
                _ => {}
            }
        }

        // 处理函数地址范围
        if let Some(name) = function_name {
            let mut function_ranges = Vec::new();

            if let (Some(low), Some(high)) = (low_pc, high_pc) {
                function_ranges.push((low, high));
            } else if let Some(ranges_offset) = ranges {
                // 处理范围列表
                let offset = dwarf.ranges_offset_from_raw(unit, ranges_offset);
                let mut range_iter = dwarf.ranges(unit, offset)?;
                while let Some(range) = range_iter.next()? {
                    function_ranges.push((range.begin, range.end));
                }
            }

            if !function_ranges.is_empty() {
                // 更新地址到位置映射中的函数名
                for (start, end) in &function_ranges {
                    for addr in *start..*end {
                        if let Some(location) = self.address_to_location.get_mut(&addr) {
                            location.function_name = Some(name.clone());
                        }
                    }
                }
                
                self.function_ranges.insert(name, function_ranges);
            }
        }

        Ok(())
    }

    /// 获取文件路径
    fn get_file_path<R: Reader>(
        &self,
        dwarf: &Dwarf<R>,
        unit: &gimli::Unit<R>,
        file: &gimli::FileEntry<R>,
    ) -> Result<String> {
        let mut path = String::new();
        
        // 获取目录
        if let Some(dir) = file.directory(unit.line_program.as_ref().unwrap().header()) {
            if let Ok(dir_name) = dwarf.attr_string(unit, dir) {
                path.push_str(&dir_name.to_string_lossy()?);
                if !path.ends_with('/') {
                    path.push('/');
                }
            }
        }
        
        // 获取文件名
        if let Ok(file_name) = dwarf.attr_string(unit, file.path_name()) {
            path.push_str(&file_name.to_string_lossy()?);
        }
        
        Ok(path)
    }

    /// 根据地址查找源码位置
    pub fn get_location(&self, address: u64) -> Option<&SourceLocation> {
        self.address_to_location.get(&address)
    }

    /// 根据地址查找最近的源码位置（用于地址不完全匹配的情况）
    pub fn get_nearest_location(&self, address: u64) -> Option<&SourceLocation> {
        // 首先尝试精确匹配
        if let Some(location) = self.address_to_location.get(&address) {
            return Some(location);
        }

        // 查找最近的较小地址
        let mut best_addr = 0;
        let mut best_location = None;
        
        for (&addr, location) in &self.address_to_location {
            if addr <= address && addr > best_addr {
                best_addr = addr;
                best_location = Some(location);
            }
        }
        
        best_location
    }

    /// 获取函数的地址范围
    pub fn get_function_ranges(&self, function_name: &str) -> Option<&Vec<(u64, u64)>> {
        self.function_ranges.get(function_name)
    }

    /// 检查地址是否在某个函数范围内
    pub fn is_address_in_function(&self, address: u64, function_name: &str) -> bool {
        if let Some(ranges) = self.function_ranges.get(function_name) {
            for (start, end) in ranges {
                if address >= *start && address < *end {
                    return true;
                }
            }
        }
        false
    }
}

impl Default for DwarfParser {
    fn default() -> Self {
        Self::new()
    }
}