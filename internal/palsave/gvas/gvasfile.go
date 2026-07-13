package gvas

// GvasFile is a fully parsed GVAS blob: header, top-level properties, trailer.
type GvasFile struct {
	Header     GvasHeader
	Properties map[string]Property
	Trailer    []byte
}

// ReadFull parses the entire GVAS blob (header + all properties + trailer),
// mirroring GvasFile.read. Used for the trailer verification gate.
func ReadFull(data []byte, hints map[string]string, custom map[string]DecodeFunc) (f GvasFile, err error) {
	r := NewReader(data, hints, custom)
	defer catch(&err)
	f.Header = r.readHeader()
	f.Properties = r.propertiesUntilEnd("")
	f.Trailer = r.ReadToEnd()
	return
}

// ReadWorldSaveData parses the header, then reads top-level properties while
// selectively decoding only the wanted children of the struct named
// structName; every other top-level property and unwanted child is skipped by
// its declared size (R4 selective parsing).
func ReadWorldSaveData(data []byte, hints map[string]string, custom map[string]DecodeFunc, structName string, wanted map[string]bool) (h GvasHeader, world map[string]Property, err error) {
	r := NewReader(data, hints, custom)
	defer catch(&err)
	h = r.readHeader()
	for {
		name := r.FString()
		if name == "None" {
			break
		}
		typeName := r.FString()
		size := int(r.U64())
		if name == structName && typeName == "StructProperty" {
			// read struct header, then filtered inner parse
			r.FString()      // struct_type
			r.Skip(16)       // struct_id
			r.OptionalGUID() // id
			world = r.readPropertiesFiltered("."+name, wanted)
		} else {
			r.skipProperty(typeName, size)
		}
	}
	return
}

// readPropertiesFiltered is the panicking core of PropertiesFilteredUntilEnd.
func (r *Reader) readPropertiesFiltered(path string, wanted map[string]bool) map[string]Property {
	props := map[string]Property{}
	for {
		name := r.FString()
		if name == "None" {
			break
		}
		typeName := r.FString()
		size := int(r.U64())
		if wanted[name] {
			props[name] = r.property(typeName, size, path+"."+name, "")
		} else {
			r.skipProperty(typeName, size)
		}
	}
	return props
}
