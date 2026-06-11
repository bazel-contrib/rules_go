package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// buildTestArchive creates a minimal .a file containing a _go_.o member with
// the given goobj Autolib imports.
func buildTestArchive(t *testing.T, dir, name string, imports []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	goobj := buildGoobj(imports)

	// AR global header.
	f.WriteString(arHeader)

	// _go_.o member header (60 bytes, fixed format).
	memberName := fmt.Sprintf("%-16s", "_go_.o")
	memberHdr := fmt.Sprintf("%s%-12s%-6s%-6s%-8s%-10d`\n",
		memberName, "0", "0", "0", "100644", len(goobj))
	f.WriteString(memberHdr)

	// goobj data.
	f.Write(goobj)

	// Pad to even boundary.
	if len(goobj)%2 != 0 {
		f.Write([]byte{'\n'})
	}

	return path
}

// buildGoobj constructs a minimal goobj binary blob with the given Autolib
// imports. Only the header, Autolib block, and string data are populated.
func buildGoobj(imports []string) []byte {
	type strEntry struct {
		s   string
		off uint32
	}

	// Header: Magic(8) + Fingerprint(8) + Flags(4) + Offsets(NBlk*4)
	headerSize := uint32(8 + 8 + 4 + goobjNBlk*4)
	autolibBlockSize := uint32(len(imports)) * goobjImportPkgSize
	strDataStart := headerSize + autolibBlockSize

	entries := make([]strEntry, len(imports))
	strOff := strDataStart
	for i, imp := range imports {
		entries[i] = strEntry{s: imp, off: strOff}
		strOff += uint32(len(imp))
	}
	totalSize := strOff

	buf := make([]byte, totalSize)

	copy(buf, goobjMagic)

	offsetsBase := 20
	autolibStart := headerSize
	autolibEnd := headerSize + autolibBlockSize
	binary.LittleEndian.PutUint32(buf[offsetsBase+0*4:], autolibStart)
	binary.LittleEndian.PutUint32(buf[offsetsBase+1*4:], autolibEnd)
	for i := 2; i < goobjNBlk; i++ {
		binary.LittleEndian.PutUint32(buf[offsetsBase+i*4:], autolibEnd)
	}

	for i, e := range entries {
		entryOff := int(autolibStart) + i*int(goobjImportPkgSize)
		binary.LittleEndian.PutUint32(buf[entryOff:], uint32(len(e.s)))
		binary.LittleEndian.PutUint32(buf[entryOff+4:], e.off)
	}

	for _, e := range entries {
		copy(buf[e.off:], e.s)
	}

	return buf
}
