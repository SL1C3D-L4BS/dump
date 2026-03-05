// Package generators: C# POCO generation from HL7/X12 inferred schema.

package generators

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CSharpTypeKind represents a C# type kind.
type CSharpTypeKind int

const (
	KindString CSharpTypeKind = iota
	KindInt
	KindDouble
	KindDateTime
	KindClass
	KindList
)

// CSharpType describes a C# type (primitive, class, or list).
type CSharpType struct {
	Kind        CSharpTypeKind
	ClassName   string   // for KindClass or element of KindList
	ElementType *CSharpType // for KindList
	Fields      []CSharpField // for KindClass
}

// CSharpField is a named property.
type CSharpField struct {
	Name  string
	Type  *CSharpType
	CSName string // C#-safe property name
}

// InferCSharpSchema builds a C# type tree from a JSON sample (e.g. from ExtractSample).
// sampleJSON is a JSON array of objects; we merge rows and infer types.
func InferCSharpSchema(sampleJSON string) (*CSharpType, error) {
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(sampleJSON), &rows); err != nil {
		return nil, fmt.Errorf("parse sample JSON: %w", err)
	}
	if len(rows) == 0 {
		return &CSharpType{Kind: KindClass, ClassName: "Root", Fields: nil}, nil
	}
	merged := mergeSampleRows(rows)
	return inferClassType("Root", merged), nil
}

// mergeSampleRows merges multiple sample rows into one structure that represents all keys and value types.
// For each key we collect value samples; for nested maps we merge recursively; we detect list by _N suffix.
func mergeSampleRows(rows []map[string]interface{}) map[string]interface{} {
	acc := make(map[string]interface{})
	for _, m := range rows {
		mergeMap(acc, m)
	}
	return acc
}

func mergeMap(acc, m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			continue
		}
		existing, ok := acc[k]
		if !ok {
			acc[k] = v
			continue
		}
		switch tv := v.(type) {
		case map[string]interface{}:
			exMap, _ := existing.(map[string]interface{})
			if exMap == nil {
				exMap = make(map[string]interface{})
				acc[k] = exMap
			}
			mergeMap(exMap, tv)
		default:
			// keep existing or merge value samples for type inference later
			acc[k] = v
		}
	}
}

// inferClassType builds a CSharpType (class) from a map. Handles nested maps and detects lists by key pattern.
func inferClassType(className string, m map[string]interface{}) *CSharpType {
	// Group keys: "OBX", "OBX_2", "OBX_3" -> one property "OBX" of type List<OBX>
	baseKeys := make(map[string][]string) // base -> all keys (e.g. OBX -> [OBX, OBX_2, OBX_3])
	var allKeys []string
	for k := range m {
		allKeys = append(allKeys, k)
		base := baseKey(k)
		baseKeys[base] = append(baseKeys[base], k)
	}
	sort.Strings(allKeys)

	var fields []CSharpField
	seenBase := make(map[string]bool)

	for _, k := range allKeys {
		base := baseKey(k)
		if seenBase[base] {
			continue
		}
		seenBase[base] = true
		keys := baseKeys[base]
		sort.Strings(keys)

		// Pick representative value (first key)
		v := m[keys[0]]
		csName := toPascal(base)

		if len(keys) > 1 {
			// Repeating segment -> List<T>
			elemType := inferFieldType(keys[0], v)
			fields = append(fields, CSharpField{
				Name:   base,
				CSName: csName,
				Type:   &CSharpType{Kind: KindList, ElementType: elemType, ClassName: elemType.ClassName},
			})
		} else {
			fields = append(fields, CSharpField{
				Name:   base,
				CSName: csName,
				Type:   inferFieldType(k, v),
			})
		}
	}
	return &CSharpType{
		Kind:      KindClass,
		ClassName: className,
		Fields:    fields,
	}
}

func baseKey(k string) string {
	// OBX_2 -> OBX, NM1_1 -> NM1
	if idx := strings.Index(k, "_"); idx > 0 {
		rest := k[idx+1:]
		if _, err := strconv.Atoi(rest); err == nil {
			return k[:idx]
		}
	}
	return k
}

func inferFieldType(key string, v interface{}) *CSharpType {
	if v == nil {
		return &CSharpType{Kind: KindString}
	}
	switch t := v.(type) {
	case map[string]interface{}:
		className := toPascal(baseKey(key))
		return inferClassType(className, t)
	case []interface{}:
		if len(t) == 0 {
			return &CSharpType{Kind: KindList, ElementType: &CSharpType{Kind: KindString}, ClassName: "object"}
		}
		elem := inferFieldType(key, t[0])
		return &CSharpType{Kind: KindList, ElementType: elem, ClassName: elem.ClassName}
	case string:
		return inferPrimitive(t)
	case float64:
		if t == float64(int64(t)) {
			return &CSharpType{Kind: KindInt}
		}
		return &CSharpType{Kind: KindDouble}
	case int, int64:
		return &CSharpType{Kind: KindInt}
	case bool:
		return &CSharpType{Kind: KindString} // C# bool; we keep string for flexibility
	default:
		return &CSharpType{Kind: KindString}
	}
}

func inferPrimitive(s string) *CSharpType {
	s = strings.TrimSpace(s)
	if s == "" {
		return &CSharpType{Kind: KindString}
	}
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &CSharpType{Kind: KindInt}
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return &CSharpType{Kind: KindDouble}
	}
	if looksLikeDateTime(s) {
		return &CSharpType{Kind: KindDateTime}
	}
	return &CSharpType{Kind: KindString}
}

var dateTimeRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}|^\d{8}|^\d{12,14}`)

func looksLikeDateTime(s string) bool {
	if len(s) < 8 {
		return false
	}
	return dateTimeRegex.MatchString(s) || strings.Contains(s, "T") || strings.Contains(s, "-")
}

// ToCSharpTypeName returns the C# type name for the given kind.
func (c *CSharpType) ToCSharpTypeName() string {
	switch c.Kind {
	case KindString:
		return "string"
	case KindInt:
		return "int"
	case KindDouble:
		return "double"
	case KindDateTime:
		return "DateTime"
	case KindClass:
		return c.ClassName
	case KindList:
		if c.ElementType != nil {
			return "List<" + c.ElementType.ToCSharpTypeName() + ">"
		}
		return "List<object>"
	default:
		return "string"
	}
}

// GenerateCSharp produces a C# POCO file content from an inferred schema.
// namespaceName is the C# namespace (e.g. "Generated.Hl7"); rootClassName is the root class name.
func GenerateCSharp(root *CSharpType, namespaceName, rootClassName string) string {
	var b strings.Builder
	b.WriteString("using System;\nusing System.Collections.Generic;\n\n")
	if namespaceName != "" {
		b.WriteString("namespace " + namespaceName + "\n{\n\n")
	}
	if root != nil && root.Kind == KindClass {
		root.ClassName = rootClassName
	}
	emitted := make(map[string]bool)
	emitClass(&b, root, emitted)
	if namespaceName != "" {
		b.WriteString("}\n")
	}
	return b.String()
}

func emitClass(b *strings.Builder, t *CSharpType, emitted map[string]bool) {
	if t == nil || t.Kind != KindClass || emitted[t.ClassName] {
		return
	}
	// Emit nested classes first (so they appear before the class that uses them)
	for _, f := range t.Fields {
		if f.Type != nil && f.Type.Kind == KindClass && !emitted[f.Type.ClassName] {
			emitClass(b, f.Type, emitted)
		}
		if f.Type != nil && f.Type.Kind == KindList && f.Type.ElementType != nil && f.Type.ElementType.Kind == KindClass && !emitted[f.Type.ElementType.ClassName] {
			emitClass(b, f.Type.ElementType, emitted)
		}
	}
	emitted[t.ClassName] = true
	b.WriteString("\tpublic class " + t.ClassName + "\n\t{\n")
	for _, f := range t.Fields {
		typeName := "string"
		if f.Type != nil {
			typeName = f.Type.ToCSharpTypeName()
		}
		b.WriteString("\t\tpublic " + typeName + " " + f.CSName + " { get; set; }\n")
	}
	b.WriteString("\t}\n\n")
}

func toPascal(s string) string {
	if s == "" {
		return s
	}
	// OBX -> Obx, segment_id -> SegmentId
	parts := strings.Split(strings.ReplaceAll(s, "_", " "), " ")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, "")
}

// InferAndGenerate runs schema inference on the sample JSON and generates C# POCO.
// Uses internal inference if needed; here we use value-based inference only.
func InferAndGenerate(sampleJSON string, namespaceName, rootClassName string) (string, error) {
	root, err := InferCSharpSchema(sampleJSON)
	if err != nil {
		return "", err
	}
	if rootClassName == "" {
		rootClassName = "Hl7Message"
	}
	if root != nil && root.Kind == KindClass {
		root.ClassName = rootClassName
	}
	return GenerateCSharp(root, namespaceName, rootClassName), nil
}

