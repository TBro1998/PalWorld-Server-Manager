package gvas

// PropertiesUntilEnd reads named properties until a "None" name is hit.
// Public wrapper with panic recovery. Mirrors properties_until_end.
func (r *Reader) PropertiesUntilEnd(path string) (props map[string]Property, err error) {
	defer catch(&err)
	props = r.propertiesUntilEnd(path)
	return
}

func (r *Reader) propertiesUntilEnd(path string) map[string]Property {
	props := map[string]Property{}
	for {
		name := r.FString()
		if name == "None" {
			break
		}
		typeName := r.FString()
		size := int(r.U64())
		props[name] = r.property(typeName, size, path+"."+name, "")
	}
	return props
}

// PropertiesFilteredUntilEnd reads properties at path but only fully decodes
// names present in wanted; others are skipped by their declared layout. Used
// for selective parsing of the large worldSaveData struct (R4). Public wrapper.
func (r *Reader) PropertiesFilteredUntilEnd(path string, wanted map[string]bool) (props map[string]Property, err error) {
	defer catch(&err)
	props = r.readPropertiesFiltered(path, wanted)
	return
}

// property decodes a single property body. Reproduces the custom_properties
// trigger and nested_caller_path semantics of FArchiveReader.property.
func (r *Reader) property(typeName string, size int, path, nestedCallerPath string) Property {
	if r.custom != nil {
		if dec, ok := r.custom[path]; ok && (nestedCallerPath != path || path == "") {
			p, err := dec(r, typeName, size, path)
			if err != nil {
				r.fail("%s", err.Error())
			}
			p.Type = typeName
			p.CustomType = path
			return p
		}
	}
	p := r.readProperty(typeName, size, path)
	p.Type = typeName
	return p
}

// Property is the exported entry a custom decoder uses to parse its own body
// generically (passing nestedCallerPath=path to avoid re-triggering itself).
func (r *Reader) Property(typeName string, size int, path, nestedCallerPath string) (p Property, err error) {
	defer catch(&err)
	p = r.property(typeName, size, path, nestedCallerPath)
	return
}

// MustProperty is the panicking variant of Property, for use inside rawdata
// decoders that already run within an Attempt or outer catch scope.
func (r *Reader) MustProperty(typeName string, size int, path, nestedCallerPath string) Property {
	return r.property(typeName, size, path, nestedCallerPath)
}

// MustProperties is the panicking variant of PropertiesUntilEnd.
func (r *Reader) MustProperties(path string) map[string]Property {
	return r.propertiesUntilEnd(path)
}

func (r *Reader) readProperty(typeName string, size int, path string) Property {
	switch typeName {
	case "StructProperty":
		return r.readStruct(path)
	case "IntProperty":
		return Property{ID: r.OptionalGUID(), Value: int64(r.I32())}
	case "UInt16Property":
		return Property{ID: r.OptionalGUID(), Value: int64(r.U16())}
	case "UInt32Property":
		return Property{ID: r.OptionalGUID(), Value: int64(r.U32())}
	case "UInt64Property":
		return Property{ID: r.OptionalGUID(), Value: int64(r.U64())}
	case "Int64Property":
		return Property{ID: r.OptionalGUID(), Value: r.I64()}
	case "FixedPoint64Property":
		return Property{ID: r.OptionalGUID(), Value: int64(r.I32())}
	case "FloatProperty":
		return Property{ID: r.OptionalGUID(), Value: r.F32()}
	case "StrProperty":
		return Property{ID: r.OptionalGUID(), Value: r.FString()}
	case "NameProperty":
		return Property{ID: r.OptionalGUID(), Value: r.FString()}
	case "EnumProperty":
		enumType := r.FString()
		id := r.OptionalGUID()
		enumValue := r.FString()
		return Property{ID: id, Value: EnumValue{Type: enumType, Value: enumValue}}
	case "BoolProperty":
		v := r.Bool()
		return Property{Value: v, ID: r.OptionalGUID()}
	case "ByteProperty":
		enumType := r.FString()
		id := r.OptionalGUID()
		if enumType == "None" {
			return Property{ID: id, Value: ByteValue{Type: enumType, Value: r.Byte()}}
		}
		return Property{ID: id, Value: ByteValue{Type: enumType, Value: r.FString()}}
	case "ArrayProperty":
		arrayType := r.FString()
		id := r.OptionalGUID()
		return Property{ArrayType: arrayType, ID: id, Value: r.arrayProperty(arrayType, size, path)}
	case "MapProperty":
		return r.readMap(size, path)
	case "SetProperty":
		return r.readSet(size, path)
	default:
		r.fail("gvas: unknown type %s (%s)", typeName, path)
		return Property{}
	}
}

func (r *Reader) readStruct(path string) Property {
	structType := r.FString()
	structID := r.GUID()
	id := r.OptionalGUID()
	value := r.structValue(structType, path)
	return Property{StructType: structType, StructID: structID, ID: id, Value: value}
}

// structValue dispatches known struct layouts; unknown types fall back to
// properties_until_end.
func (r *Reader) structValue(structType, path string) any {
	switch structType {
	case "Vector":
		return map[string]float64{"x": r.F64(), "y": r.F64(), "z": r.F64()}
	case "DateTime":
		return r.U64()
	case "Guid":
		return r.GUID()
	case "Quat":
		return map[string]float64{"x": r.F64(), "y": r.F64(), "z": r.F64(), "w": r.F64()}
	case "LinearColor":
		return map[string]float64{"r": r.F32(), "g": r.F32(), "b": r.F32(), "a": r.F32()}
	case "Color":
		return map[string]uint8{"b": r.Byte(), "g": r.Byte(), "r": r.Byte(), "a": r.Byte()}
	default:
		return r.propertiesUntilEnd(path)
	}
}

func (r *Reader) arrayProperty(arrayType string, size int, path string) ArrayValue {
	count := int(r.U32())
	if arrayType == "StructProperty" {
		propName := r.FString()
		propType := r.FString()
		r.U64()
		typeName := r.FString()
		id := r.GUID()
		r.Skip(1)
		values := make([]any, 0, count)
		for i := 0; i < count; i++ {
			values = append(values, r.structValue(typeName, path+"."+propName))
		}
		return ArrayValue{PropName: propName, PropType: propType, TypeName: typeName, ID: id, Values: values}
	}
	return ArrayValue{Values: r.arrayValue(arrayType, count, size-4, path)}
}

func (r *Reader) arrayValue(arrayType string, count, size int, path string) any {
	switch arrayType {
	case "EnumProperty", "NameProperty":
		out := make([]string, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, r.FString())
		}
		return out
	case "Guid":
		out := make([]string, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, r.GUID())
		}
		return out
	case "ByteProperty":
		if size == count {
			return r.ByteList(count)
		}
		r.fail("gvas: labelled ByteProperty not implemented (%s)", path)
		return nil
	default:
		r.fail("gvas: unknown array type %s (%s)", arrayType, path)
		return nil
	}
}

func (r *Reader) readMap(size int, path string) Property {
	keyType := r.FString()
	valueType := r.FString()
	id := r.OptionalGUID()
	r.U32()
	count := int(r.U32())

	keyPath := path + ".Key"
	keyStructType := ""
	if keyType == "StructProperty" {
		keyStructType = r.getTypeOr(keyPath, "Guid")
	}
	valuePath := path + ".Value"
	valueStructType := ""
	if valueType == "StructProperty" {
		valueStructType = r.getTypeOr(valuePath, "StructProperty")
	}

	values := make([]MapEntry, 0, count)
	for i := 0; i < count; i++ {
		k := r.propValue(keyType, keyStructType, keyPath)
		v := r.propValue(valueType, valueStructType, valuePath)
		values = append(values, MapEntry{Key: k, Value: v})
	}
	return Property{
		KeyType: keyType, ValueType: valueType,
		KeyStructType: keyStructType, ValueStructType: valueStructType,
		ID: id, Value: values,
	}
}

func (r *Reader) readSet(size int, path string) Property {
	setType := r.FString()
	id := r.OptionalGUID()
	r.U32()
	count := int(r.U32())
	structType := ""
	values := make([]any, 0, count)
	if setType == "StructProperty" {
		structType = r.getTypeOr(path+".StructProperty", "StructProperty")
		for i := 0; i < count; i++ {
			values = append(values, r.structValue(structType, path+".StructProperty"))
		}
	} else {
		for i := 0; i < count; i++ {
			values = append(values, r.propertiesUntilEnd(path))
		}
	}
	return Property{SetType: setType, StructType: structType, ID: id, Value: SetValue{StructType: structType, Values: values}}
}

// propValue reads a bare value inside a Map/Set. Mirrors prop_value.
func (r *Reader) propValue(typeName, structTypeName, path string) any {
	switch typeName {
	case "StructProperty":
		return r.structValue(structTypeName, path)
	case "EnumProperty":
		return r.FString()
	case "NameProperty":
		return r.FString()
	case "IntProperty":
		return int64(r.I32())
	case "BoolProperty":
		return r.Bool()
	case "UInt32Property":
		return int64(r.U32())
	case "StrProperty":
		return r.FString()
	case "Int64Property":
		return r.I64()
	default:
		r.fail("gvas: unknown property value type %s (%s)", typeName, path)
		return nil
	}
}

// skipProperty consumes a property body without decoding it, for selective
// parsing. It reads the type-specific header that precedes the sized region,
// then seeks past `size` bytes. The header/size split matches the reference
// writer: `size` covers only the region after each type's own header.
func (r *Reader) skipProperty(typeName string, size int) {
	switch typeName {
	case "StructProperty":
		r.FString()      // struct_type
		r.Skip(16)       // struct_id
		r.OptionalGUID() // id
		r.Skip(size)     // struct_value
	case "MapProperty":
		r.FString()      // key_type
		r.FString()      // value_type
		r.OptionalGUID() // id
		r.Skip(size)
	case "ArrayProperty":
		r.FString()      // array_type
		r.OptionalGUID() // id
		r.Skip(size)
	case "SetProperty":
		r.FString()      // set_type
		r.OptionalGUID() // id
		r.Skip(size)
	case "EnumProperty", "ByteProperty":
		r.FString()      // enum/byte type
		r.OptionalGUID() // id
		r.Skip(size)
	case "BoolProperty":
		r.Skip(1)        // bool value (size is 0)
		r.OptionalGUID() // id
	case "IntProperty", "UInt16Property", "UInt32Property", "UInt64Property",
		"Int64Property", "FixedPoint64Property", "FloatProperty",
		"StrProperty", "NameProperty":
		r.OptionalGUID()
		r.Skip(size)
	default:
		r.fail("gvas: cannot skip unknown type %s", typeName)
	}
}
