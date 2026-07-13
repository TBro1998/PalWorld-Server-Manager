// Package gvas is a pure-Go, read-only reader for Unreal Engine GVAS save
// blobs, faithfully ported from the read path of temp/palsav/palsav/archive.py
// and gvas.py. It contains no Palworld-specific logic: Palworld type hints and
// custom property decoders are injected by the parent palsave package.
//
// This package never writes GVAS data.
package gvas

// Property is the decoded representation of a single UE property, mirroring the
// dict produced by FArchiveReader.property in the reference implementation.
// Only the fields relevant to a given Type are populated.
type Property struct {
	Type string // e.g. "IntProperty", "StructProperty", "MapProperty"
	ID   *string

	// Value holds the type-specific payload:
	//   IntProperty/UInt*/Int64/FixedPoint64  -> int64
	//   FloatProperty                          -> float64
	//   StrProperty/NameProperty               -> string
	//   BoolProperty                           -> bool
	//   EnumProperty                           -> EnumValue
	//   ByteProperty                           -> ByteValue
	//   StructProperty                         -> struct value (see structValue)
	//   ArrayProperty                          -> ArrayValue
	//   MapProperty                            -> []MapEntry
	//   SetProperty                            -> SetValue
	// Custom decoders replace Value with their own decoded payload (any).
	Value any

	// StructProperty
	StructType string
	StructID   string

	// ArrayProperty
	ArrayType string

	// MapProperty
	KeyType         string
	ValueType       string
	KeyStructType   string
	ValueStructType string

	// SetProperty
	SetType string

	// Set to the property path when a custom decoder produced this property.
	CustomType string
}

// EnumValue is the payload of an EnumProperty (and the labelled ByteProperty).
type EnumValue struct {
	Type  string
	Value string
}

// ByteValue is the payload of a ByteProperty. When Type == "None", Value is a
// uint8; otherwise Value is a string (enum member name).
type ByteValue struct {
	Type  string
	Value any
}

// MapEntry is one key/value pair of a MapProperty.
type MapEntry struct {
	Key   any
	Value any
}

// ArrayValue is the payload of an ArrayProperty.
//
// For a plain array, Values holds:
//   - []byte   for ByteProperty
//   - []string for EnumProperty/NameProperty/Guid
//
// For a StructProperty array, the header fields are populated and Values holds
// a []any of struct values.
type ArrayValue struct {
	PropName string
	PropType string
	TypeName string
	ID       string
	Values   any
}

// SetValue is the payload of a SetProperty.
type SetValue struct {
	StructType string
	Values     []any
}

// GvasHeader is the parsed GVAS file header.
type GvasHeader struct {
	Magic                   int32
	SaveGameVersion         int32
	PackageFileVersionUE4   int32
	PackageFileVersionUE5   int32
	EngineVersionMajor      uint16
	EngineVersionMinor      uint16
	EngineVersionPatch      uint16
	EngineVersionChangelist uint32
	EngineVersionBranch     string
	CustomVersionFormat     int32
	CustomVersions          []CustomVersion
	SaveGameClassName       string
}

// CustomVersion is a (guid, version) pair from the GVAS header.
type CustomVersion struct {
	GUID    string
	Version int32
}
