// Minimal reader for Go object files (.a archives containing goobj-format
// _go_.o members). Extracts the Autolib block (list of directly imported
// packages) from the goobj header without depending on cmd/internal/goobj.
//
// Binary layout reference: cmd/internal/goobj/objfile.go (Go 1.20+)
//
//	Header:
//	  Magic       "\x00go120ld"  (8 bytes)
//	  Fingerprint [8]byte
//	  Flags       uint32
//	  Offsets     [NBlk]uint32   (NBlk = 20)
//
//	Autolib block (Offsets[0]..Offsets[1]):
//	  []ImportedPkg, each 16 bytes:
//	    Pkg         StringRef (8 bytes: length uint32 LE + offset uint32 LE)
//	    Fingerprint [8]byte
//
//	Strings block starts at Offsets[NBlk-1] (index 19) — but string data is
//	stored starting at the very end of the header, so StringRef offsets are
//	relative to the start of the goobj data (= start of _go_.o member).

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	goobjMagic         = "\x00go120ld"
	goobjNBlk          = 20
	goobjStringRefSize = 8
	goobjImportPkgSize = goobjStringRefSize + 8 // StringRef + Fingerprint
)

// readAutolibImports opens an ar archive, finds the _go_.o member, parses its
// goobj header, and returns the list of directly imported package paths.
func readAutolibImports(archivePath string) ([]string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := findGoObjMember(f)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	return parseAutolibFromGoobj(data)
}

// findGoObjMember scans an ar archive for the __.PKGDEF-terminated header area
// then finds the _go_.o member (or the first member after __.PKGDEF).
func findGoObjMember(f *os.File) ([]byte, error) {
	magic := make([]byte, len(arHeader))
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if string(magic) != arHeader {
		return nil, fmt.Errorf("not an ar archive")
	}

	for {
		hdr := &header{}
		if err := binary.Read(f, binary.BigEndian, hdr); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, nil
			}
			return nil, err
		}

		name := hdr.name()
		size := hdr.size()

		if name == "__.PKGDEF" || name == "__.PKGDEF/" {
			// Skip the package definition; the _go_.o follows.
			if _, err := f.Seek(hdr.next(), io.SeekCurrent); err != nil {
				return nil, err
			}
			continue
		}

		if name == "_go_.o" || name == "_go_.o/" {
			data := make([]byte, size)
			if _, err := io.ReadFull(f, data); err != nil {
				return nil, err
			}
			return data, nil
		}

		// Skip unknown members.
		if _, err := f.Seek(hdr.next(), io.SeekCurrent); err != nil {
			return nil, err
		}
	}
}

// parseAutolibFromGoobj extracts imported package names from raw goobj bytes.
func parseAutolibFromGoobj(data []byte) ([]string, error) {
	magicLen := len(goobjMagic)
	if len(data) < magicLen {
		return nil, fmt.Errorf("goobj too short")
	}
	if string(data[:magicLen]) != goobjMagic {
		return nil, fmt.Errorf("bad goobj magic: %q", data[:magicLen])
	}

	// Header layout: Magic(8) + Fingerprint(8) + Flags(4) + Offsets(NBlk * 4)
	headerSize := magicLen + 8 + 4 + goobjNBlk*4
	if len(data) < headerSize {
		return nil, fmt.Errorf("goobj header truncated")
	}

	offsetsStart := magicLen + 8 + 4
	offsets := make([]uint32, goobjNBlk)
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(data[offsetsStart+i*4:])
	}

	// BlkAutolib = 0: Offsets[0] is start, Offsets[1] is end.
	autolibStart := offsets[0]
	autolibEnd := offsets[1]
	if autolibEnd < autolibStart {
		return nil, nil
	}
	count := (autolibEnd - autolibStart) / goobjImportPkgSize
	if count == 0 {
		return nil, nil
	}

	imports := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		entryOff := autolibStart + i*goobjImportPkgSize
		if int(entryOff+goobjStringRefSize) > len(data) {
			break
		}
		strLen := binary.LittleEndian.Uint32(data[entryOff:])
		strOff := binary.LittleEndian.Uint32(data[entryOff+4:])
		if int(strOff+strLen) > len(data) {
			break
		}
		imports = append(imports, string(data[strOff:strOff+strLen]))
	}
	return imports, nil
}
