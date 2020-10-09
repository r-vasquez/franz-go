package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var maxKey int

func die(why string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, why+"\n", args...)
	os.Exit(1)
}

type (
	// LineWriter writes lines at a time.
	LineWriter struct {
		buf  *bytes.Buffer
		line int
	}

	Type interface {
		WriteAppend(*LineWriter)
		WriteDecode(*LineWriter)
		TypeName() string
	}

	Bool struct {
		HasDefault bool
		Default    bool
	}
	Int8 struct {
		HasDefault bool
		Default    int8
	}
	Int16 struct {
		HasDefault bool
		Default    int16
	}
	Int32 struct {
		HasDefault bool
		Default    int32
	}
	Int64 struct {
		HasDefault bool
		Default    int64
	}
	Float64 struct {
		HasDefault bool
		Default    float64
	}
	Uint32 struct {
		HasDefault bool
		Default    uint32
	}
	Varint struct {
		HasDefault bool
		Default    int32
	}
	VarintString struct{}
	VarintBytes  struct{}

	FieldLengthMinusBytes struct {
		Field       string
		LengthMinus int
	}

	// The following types can be encoded "compact"; this happens on
	// flexible versions. If adding types here, be sure to add the
	// AsFromFlexible method below.
	String struct {
		FromFlexible bool
	}
	NullableString struct {
		HasDefault      bool
		FromFlexible    bool
		NullableVersion int
	}
	Bytes struct {
		FromFlexible bool
	}
	NullableBytes struct {
		HasDefault   bool
		FromFlexible bool
	}

	Array struct {
		Inner           Type
		IsVarintArray   bool
		IsNullableArray bool
		NullableVersion int

		HasDefault bool

		// FromFlexible is true if this is inside a struct that has
		// flexible versions.
		FromFlexible bool
	}

	StructField struct {
		Comment    string
		MinVersion int
		MaxVersion int
		Tag        int
		FieldName  string
		Type       Type
	}

	Struct struct {
		TopLevel         bool
		WithVersionField bool // if not top level
		WithNoEncoding   bool // if not top level
		Anonymous        bool // if inner struct
		Comment          string
		Name             string

		HasDefault bool

		// FromFlexible tracks if this struct is either
		// (a) top level and has flexible versions, or
		// (b) nested in a top level struct that has flexible versions
		FromFlexible bool

		Fields []StructField

		Key int // -1 if not top level

		// Only TopLevel relevant fields:
		Admin            bool
		GroupCoordinator bool
		TxnCoordinator   bool
		MaxVersion       int
		FlexibleAt       int
		ResponseKind     string // for requests
		RequestKind      string // for responses
	}
)

/////////////////////
// DEFUALT SETTING //
/////////////////////

type Defaulter interface {
	SetDefault(string) Type
	GetDefault() (interface{}, bool)
	GetTypeDefault() interface{}
}

func (b Bool) SetDefault(s string) Type {
	v, err := strconv.ParseBool(s)
	if err != nil {
		die("invalid bool default: %v", err)
	}
	b.Default = v
	b.HasDefault = true
	return b
}
func (b Bool) GetDefault() (interface{}, bool) { return b.Default, b.HasDefault }
func (b Bool) GetTypeDefault() interface{}     { return false }

func (i Int8) SetDefault(s string) Type {
	v, err := strconv.ParseInt(s, 10, 8)
	if err != nil {
		die("invalid int8 default: %v", err)
	}
	i.Default = int8(v)
	i.HasDefault = true
	return i
}
func (i Int8) GetDefault() (interface{}, bool) { return i.Default, i.HasDefault }
func (i Int8) GetTypeDefault() interface{}     { return 0 }

func (i Int16) SetDefault(s string) Type {
	v, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		die("invalid int16 default: %v", err)
	}
	i.Default = int16(v)
	i.HasDefault = true
	return i
}
func (i Int16) GetDefault() (interface{}, bool) { return i.Default, i.HasDefault }
func (i Int16) GetTypeDefault() interface{}     { return 0 }

func (i Int32) SetDefault(s string) Type {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		die("invalid int32 default: %v", err)
	}
	i.Default = int32(v)
	i.HasDefault = true
	return i
}
func (i Int32) GetDefault() (interface{}, bool) { return i.Default, i.HasDefault }
func (i Int32) GetTypeDefault() interface{}     { return 0 }

func (i Int64) SetDefault(s string) Type {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		die("invalid int64 default: %v", err)
	}
	i.Default = int64(v)
	i.HasDefault = true
	return i
}
func (i Int64) GetDefault() (interface{}, bool) { return i.Default, i.HasDefault }
func (i Int64) GetTypeDefault() interface{}     { return 0 }

func (f Float64) SetDefault(s string) Type {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		die("invalid float64 default: %v", err)
	}
	f.Default = v
	f.HasDefault = true
	return f
}
func (f Float64) GetDefault() (interface{}, bool) { return f.Default, f.HasDefault }
func (f Float64) GetTypeDefault() interface{}     { return 0 }

func (u Uint32) SetDefault(s string) Type {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		die("invalid uint32 default: %v", err)
	}
	u.Default = uint32(v)
	u.HasDefault = true
	return u
}
func (u Uint32) GetDefault() (interface{}, bool) { return u.Default, u.HasDefault }
func (u Uint32) GetTypeDefault() interface{}     { return 0 }

func (i Varint) SetDefault(s string) Type {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		die("invalid varint default: %v", err)
	}
	i.Default = int32(v)
	i.HasDefault = true
	return i
}
func (i Varint) GetDefault() (interface{}, bool) { return i.Default, i.HasDefault }
func (i Varint) GetTypeDefault() interface{}     { return 0 }

func (s NullableString) SetDefault(v string) Type {
	if v != "null" {
		die("unknown non-null default for nullable string")
	}
	s.HasDefault = true
	return s
}
func (s NullableString) GetDefault() (interface{}, bool) {
	return "nil", s.HasDefault // we return the string so it is rendered correctly
}
func (s NullableString) GetTypeDefault() interface{} { return nil }

func (b NullableBytes) SetDefault(v string) Type {
	if v != "null" {
		die("unknown non-null default for nullable string")
	}
	b.HasDefault = true
	return b
}
func (b NullableBytes) GetDefault() (interface{}, bool) {
	return "nil", b.HasDefault
}
func (b NullableBytes) GetTypeDefault() interface{} { return nil }

func (a Array) SetDefault(v string) Type {
	if v != "null" {
		die("unknown non-null default for array")
	}
	a.HasDefault = true
	return a
}
func (a Array) GetDefault() (interface{}, bool) {
	return "nil", a.HasDefault
}
func (a Array) GetTypeDefault() interface{} { return "nil" }

func (s Struct) SetDefault(v string) Type {
	die("cannot set default on a struct; we already have a default")
	return s
}
func (s Struct) GetDefault() (interface{}, bool) {
	return "", false // no GetDefault
}
func (s Struct) GetTypeDefault() interface{} {
	// This will not work if a tagged type has its own arrays, but for now
	// nothing has that.
	return fmt.Sprintf("(func() %[1]s { var v %[1]s; v.Default(); return v })() ", s.Name)
}

type FlexibleSetter interface {
	AsFromFlexible() Type
}

func (s String) AsFromFlexible() Type         { dup := s; dup.FromFlexible = true; return dup }
func (s NullableString) AsFromFlexible() Type { dup := s; dup.FromFlexible = true; return dup }
func (s Bytes) AsFromFlexible() Type          { dup := s; dup.FromFlexible = true; return dup }
func (s NullableBytes) AsFromFlexible() Type  { dup := s; dup.FromFlexible = true; return dup }
func (s Array) AsFromFlexible() Type          { dup := s; dup.FromFlexible = true; return dup }
func (s Struct) AsFromFlexible() Type         { dup := s; dup.FromFlexible = true; return dup }

func (l *LineWriter) Write(line string, args ...interface{}) {
	fmt.Fprintf(l.buf, line, args...)
	l.buf.WriteByte('\n')
	l.line++
}

//go:generate sh -c "go run . | gofmt > ../pkg/kmsg/generated.go"
func main() {
	const dir = "definitions"
	dirents, err := ioutil.ReadDir(dir)
	if err != nil {
		die("unable to read definitions dir %s: %v", dir, err)
	}
	for _, ent := range dirents {
		if strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		f, err := ioutil.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			die("unable to read %s/%s: %v", dir, ent.Name(), err)
		}
		Parse(f)
	}

	l := &LineWriter{buf: bytes.NewBuffer(make([]byte, 0, 300<<10))}
	l.Write("package kmsg")
	l.Write("import (")
	l.Write(`"context"`)
	l.Write("")
	l.Write(`"github.com/twmb/kafka-go/pkg/kbin"`)
	l.Write(")")
	l.Write("// Code generated by kafka-go/generate. DO NOT EDIT.\n")

	l.Write("// MaxKey is the maximum key used for any messages in this package.")
	l.Write("// Note that this value will change as Kafka adds more messages.")
	l.Write("const MaxKey = %d\n", maxKey)

	var name2structs []Struct

	sort.SliceStable(newStructs, func(i, j int) bool { return newStructs[i].Key < newStructs[j].Key })
	for _, s := range newStructs {
		s.WriteDefn(l)
		if s.TopLevel {
			if s.ResponseKind != "" {
				name2structs = append(name2structs, s)
			}

			s.WriteKeyFunc(l)
			s.WriteMaxVersionFunc(l)
			s.WriteSetVersionFunc(l)
			s.WriteGetVersionFunc(l)
			s.WriteIsFlexibleFunc(l)

			if s.ResponseKind != "" {
				if s.Admin {
					s.WriteAdminFunc(l)
				} else if s.GroupCoordinator {
					s.WriteGroupCoordinatorFunc(l)
				} else if s.TxnCoordinator {
					s.WriteTxnCoordinatorFunc(l)
				}
				s.WriteResponseKindFunc(l)
				s.WriteRequestWithFunc(l)
			}
			if s.RequestKind != "" {
				s.WriteRequestKindFunc(l)
			}

			l.Write("") // newline before append/decode func
			s.WriteAppendFunc(l)
			s.WriteDecodeFunc(l)
			s.WriteNewPtrFunc(l)
		} else if !s.Anonymous && !s.WithNoEncoding {
			s.WriteAppendFunc(l)
			s.WriteDecodeFunc(l)
		}

		// everything gets a default and new function
		s.WriteDefaultFunc(l)
		s.WriteNewFunc(l)
	}

	l.Write("// RequestForKey returns the request corresponding to the given request key")
	l.Write("// or nil if the key is unknown.")
	l.Write("func RequestForKey(key int16) Request {")
	l.Write("switch key {")
	l.Write("default: return nil")
	for _, key2struct := range name2structs {
		l.Write("case %d: return new(%s)", key2struct.Key, key2struct.Name)
	}
	l.Write("}")
	l.Write("}")

	l.Write("// ResponseForKey returns the response corresponding to the given request key")
	l.Write("// or nil if the key is unknown.")
	l.Write("func ResponseForKey(key int16) Response {")
	l.Write("switch key {")
	l.Write("default: return nil")
	for _, key2struct := range name2structs {
		l.Write("case %d: return new(%s)", key2struct.Key, strings.TrimSuffix(key2struct.Name, "Request")+"Response")
	}
	l.Write("}")
	l.Write("}")

	l.Write("// NameForKey returns the name (e.g., \"Fetch\") corresponding to a given request key")
	l.Write("// or \"\" if the key is unknown.")
	l.Write("func NameForKey(key int16) string {")
	l.Write("switch key {")
	l.Write("default: return \"\"")
	for _, key2struct := range name2structs {
		l.Write("case %d: return \"%s\"", key2struct.Key, strings.TrimSuffix(key2struct.Name, "Request"))
	}
	l.Write("}")
	l.Write("}")

	fmt.Println(l.buf.String())
}
