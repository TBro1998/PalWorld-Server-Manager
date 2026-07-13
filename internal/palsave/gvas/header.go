package gvas

// gvasMagic is the little-endian "GVAS" magic (0x53415647).
const gvasMagic = 1396790855

// ReadHeader parses the GVAS header. Mirrors GvasHeader.read.
func (r *Reader) ReadHeader() (h GvasHeader, err error) {
	defer catch(&err)
	h = r.readHeader()
	return
}

func (r *Reader) readHeader() GvasHeader {
	var h GvasHeader
	h.Magic = r.I32()
	if h.Magic != gvasMagic {
		r.fail("gvas: invalid magic %d", h.Magic)
	}
	h.SaveGameVersion = r.I32()
	if h.SaveGameVersion != 3 {
		r.fail("gvas: expected save game version 3, got %d", h.SaveGameVersion)
	}
	h.PackageFileVersionUE4 = r.I32()
	h.PackageFileVersionUE5 = r.I32()
	h.EngineVersionMajor = r.U16()
	h.EngineVersionMinor = r.U16()
	h.EngineVersionPatch = r.U16()
	h.EngineVersionChangelist = r.U32()
	h.EngineVersionBranch = r.FString()
	h.CustomVersionFormat = r.I32()
	if h.CustomVersionFormat != 3 {
		r.fail("gvas: expected custom version format 3, got %d", h.CustomVersionFormat)
	}
	count := r.U32()
	h.CustomVersions = make([]CustomVersion, 0, count)
	for i := uint32(0); i < count; i++ {
		h.CustomVersions = append(h.CustomVersions, CustomVersion{GUID: r.GUID(), Version: r.I32()})
	}
	h.SaveGameClassName = r.FString()
	return h
}
