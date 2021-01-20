// +build go1.8
// +build !go1.16

package goloader

import (
	"cmd/objfile/goobj"
	"fmt"
	"os"
)

var (
	x86moduleHead = []byte{0xFB, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x1, PtrSize}
	armmoduleHead = []byte{0xFB, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x4, PtrSize}
)

func Parse(f *os.File, pkgpath *string) ([]string, error) {
	obj, err := goobj.Parse(f, *pkgpath)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}
	symbolNames := make([]string, 0)
	for _, sym := range obj.Syms {
		symbolNames = append(symbolNames, sym.Name)
	}
	return symbolNames, nil
}

func (pkg *Pkg) symbols() error {
	obj, err := goobj.Parse(pkg.f, pkg.PkgPath)
	if err != nil {
		return fmt.Errorf("read error: %v", err)
	}
	pkg.Arch = obj.Arch
	fd := readAtSeeker{ReadSeeker: pkg.f}
	for _, sym := range obj.Syms {
		symbol := &ObjSymbol{}
		symbol.Name = sym.Name
		symbol.Kind = int(sym.Kind)
		symbol.DupOK = sym.DupOK
		symbol.Size = int64(sym.Size)
		symbol.Data, err = fd.BytesAt(sym.Data.Offset, sym.Data.Size)
		if err != nil {
			return fmt.Errorf("read error: %v", err)
		}
		grow(&symbol.Data, (int)(symbol.Size))
		for _, loc := range sym.Reloc {
			reloc := Reloc{
				Offset: int(loc.Offset),
				Sym:    &Sym{Name: loc.Sym.Name, Offset: InvalidOffset},
				Type:   int(loc.Type),
				Size:   int(loc.Size),
				Add:    int(loc.Add)}
			symbol.Reloc = append(symbol.Reloc, reloc)
		}
		if sym.Func != nil {
			symbol.Func = &FuncInfo{}
			symbol.Func.Args = uint32(sym.Func.Args)
			symbol.Func.File = sym.Func.File
			symbol.Func.PCSP, err = fd.BytesAt(sym.Func.PCSP.Offset, sym.Func.PCSP.Size)
			if err != nil {
				return fmt.Errorf("read error: %v", err)
			}
			symbol.Func.PCFile, err = fd.BytesAt(sym.Func.PCFile.Offset, sym.Func.PCFile.Size)
			if err != nil {
				return fmt.Errorf("read error: %v", err)
			}
			symbol.Func.PCLine, err = fd.BytesAt(sym.Func.PCLine.Offset, sym.Func.PCLine.Size)
			if err != nil {
				return fmt.Errorf("read error: %v", err)
			}

			for _, data := range sym.Func.PCData {
				pcdata, err := fd.BytesAt(data.Offset, data.Size)
				if err != nil {
					return fmt.Errorf("read error: %v", err)
				}
				symbol.Func.PCData = append(symbol.Func.PCData, pcdata)
			}
			for _, data := range sym.Func.FuncData {
				symbol.Func.FuncData = append(symbol.Func.FuncData, data.Sym.Name)
			}

			if err = initInline(sym.Func, symbol.Func, pkg.PkgPath, &fd); err != nil {
				return fmt.Errorf("read error: %v", err)
			}
		}
		pkg.Syms[sym.Name] = symbol
	}
	return nil
}

func initCodeReloc() *CodeReloc {
	reloc := &CodeReloc{symMap: make(map[string]*Sym),
		stkmaps: make(map[string][]byte),
		namemap: make(map[string]int),
	}
	reloc.pclntable = append(reloc.pclntable, x86moduleHead...)
	return reloc
}
