package gate

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// ReadableObservedValue renders JSON5 without losing exact evidence values.
func ReadableObservedValue(value *ontology.ObservedValue) string {
	var out strings.Builder
	writeJSON5Value(&out, value, 0, false)
	return out.String()
}

// ErrOpaqueDocumentBytes identifies a valid byte value that cannot be presented
// as semantic JSON5. Reader documents must explain its byte type and link to the
// exact evidence instead of inventing a null or string domain value.
var ErrOpaqueDocumentBytes = errors.New("non-UTF-8 bytes require an opaque byte explanation and evidence link")

// ReadableObservedValueForDocument renders valid semantic JSON5 without
// technical byte/bit/offset diagnostics. Malformed values fail closed; valid
// non-UTF-8 bytes return ErrOpaqueDocumentBytes, including nested byte values.
func ReadableObservedValueForDocument(value *ontology.ObservedValue) (string, error) {
	if value == nil {
		return "", fmt.Errorf("document value is absent")
	}
	if err := value.Validate(); err != nil {
		return "", fmt.Errorf("invalid document value: %w", err)
	}
	var out strings.Builder
	if err := writeJSON5Value(&out, value, 0, true); err != nil {
		return "", err
	}
	return out.String(), nil
}

func json5String(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func writeJSON5Value(out *strings.Builder, value *ontology.ObservedValue, depth int, document bool) error {
	if value == nil {
		out.WriteString("/* absent */ null")
		return nil
	}
	switch value.Kind {
	case "null":
		out.WriteString("null")
	case "text":
		if value.Text == nil {
			out.WriteString("/* absent text */ null")
		} else {
			out.WriteString(json5String(*value.Text))
		}
	case "bool":
		if value.Bool == nil {
			out.WriteString("/* absent bool */ null")
		} else {
			out.WriteString(strconv.FormatBool(*value.Bool))
		}
	case "integer":
		number, err := strconv.ParseInt(value.Integer, 10, 64)
		if err != nil || number < -9007199254740991 || number > 9007199254740991 {
			out.WriteString("/* Integer: exact decimal */ ")
			out.WriteString(json5String(value.Integer))
		} else {
			out.WriteString(value.Integer)
		}
	case "float":
		bits, err := strconv.ParseUint(value.FloatBits, 16, 64)
		if err != nil {
			out.WriteString("/* invalid Float bits */ ")
			out.WriteString(json5String(value.FloatBits))
			return nil
		}
		if !document {
			out.WriteString("/* Float bits: " + value.FloatBits + " */ ")
		}
		number := math.Float64frombits(bits)
		switch {
		case math.IsNaN(number):
			out.WriteString("NaN")
		case math.IsInf(number, 1):
			out.WriteString("Infinity")
		case math.IsInf(number, -1):
			out.WriteString("-Infinity")
		default:
			text := strconv.FormatFloat(number, 'g', -1, 64)
			if !strings.ContainsAny(text, ".eE") {
				text += ".0"
			}
			out.WriteString(text)
		}
	case "bytes":
		bytes, err := base64.StdEncoding.Strict().DecodeString(value.Bytes)
		if err != nil {
			out.WriteString("/* undecoded base64 bytes */ " + json5String(value.Bytes))
		} else if utf8.Valid(bytes) {
			out.WriteString(json5String(string(bytes)))
		} else if document {
			return ErrOpaqueDocumentBytes
		} else {
			out.WriteString("/* bytes: hexadecimal, invalid UTF-8 */ " + json5String(fmt.Sprintf("% X", bytes)))
		}
	case "object":
		return writeJSON5Object(out, value.Fields, depth, document)
	case "array":
		return writeJSON5Array(out, value.Items, depth, document)
	case "diagnostic":
		if value.Diagnostic == nil {
			out.WriteString("/* absent diagnostic */ null")
		} else {
			diagnostic := *value.Diagnostic
			if document {
				diagnostic.Line, diagnostic.Span = nil, nil
			}
			encoded, _ := json.MarshalIndent(diagnostic, strings.Repeat("  ", depth), "  ")
			out.Write(encoded)
		}
	default:
		out.WriteString("/* unrepresented evidence kind */ " + json5String(value.Kind))
	}
	return nil
}

func writeJSON5Object(out *strings.Builder, fields map[string]ontology.ObservedValue, depth int, document bool) error {
	if len(fields) == 0 {
		out.WriteString("{}")
		return nil
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out.WriteString("{\n")
	for index, key := range keys {
		out.WriteString(strings.Repeat("  ", depth+1))
		if json5Identifier(key) {
			out.WriteString(key)
		} else {
			out.WriteString(json5String(key))
		}
		out.WriteString(": ")
		item := fields[key]
		if err := writeJSON5Value(out, &item, depth+1, document); err != nil {
			return err
		}
		if index+1 < len(keys) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString(strings.Repeat("  ", depth) + "}")
	return nil
}

func writeJSON5Array(out *strings.Builder, items []ontology.ObservedValue, depth int, document bool) error {
	if len(items) == 0 {
		out.WriteString("[]")
		return nil
	}
	out.WriteString("[\n")
	for index := range items {
		out.WriteString(strings.Repeat("  ", depth+1))
		if err := writeJSON5Value(out, &items[index], depth+1, document); err != nil {
			return err
		}
		if index+1 < len(items) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString(strings.Repeat("  ", depth) + "]")
	return nil
}

func readableSourceBytes(value *ontology.ObservedValue) (string, bool) {
	if value == nil || value.Kind != "bytes" {
		return "", false
	}
	bytes, err := base64.StdEncoding.DecodeString(value.Bytes)
	if err != nil || !utf8.Valid(bytes) {
		return "", false
	}
	return string(bytes), true
}

func json5Identifier(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		start := char == '$' || char == '_' || unicode.IsLetter(char) || unicode.Is(unicode.Nl, char) || unicode.Is(unicode.Other_ID_Start, char)
		if start {
			continue
		}
		if index == 0 || !(unicode.Is(unicode.Mn, char) || unicode.Is(unicode.Mc, char) || unicode.Is(unicode.Nd, char) || unicode.Is(unicode.Pc, char) || unicode.Is(unicode.Other_ID_Continue, char) || char == '\u200c' || char == '\u200d') {
			return false
		}
	}
	return true
}

func sourceCodePointNotes(text string) string {
	var notes strings.Builder
	for offset, char := range text {
		if char == '\n' || char == '\t' || char == ' ' || unicode.IsPrint(char) && !unicode.IsSpace(char) {
			continue
		}
		if notes.Len() > 0 {
			notes.WriteString(", ")
		}
		fmt.Fprintf(&notes, "%d=U+%04X", offset, char)
	}
	return notes.String()
}
