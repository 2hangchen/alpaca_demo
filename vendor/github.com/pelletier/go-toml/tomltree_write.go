package toml

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type valueComplexity int

const (
	valueSimple valueComplexity = iota + 1
	valueComplex
)

type sortNode struct {
	key        string
	complexity valueComplexity
}

// Encodes a string to a TOML-compliant multi-line string value
// This function is a clone of the existing encodeTomlString function, except that whitespace characters
// are preserved. Quotation marks and backslashes are also not escaped.
func encodeMultilineTomlString(value string) string {
	var b bytes.Buffer

	for _, rr := range value {
		switch rr {
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString("\t")
		case '\n':
			b.WriteString("\n")
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString("\r")
		case '"':
			b.WriteString(`"`)
		case '\\':
			b.WriteString(`\`)
		default:
			intRr := uint16(rr)
			if intRr < 0x001F {
				b.WriteString(fmt.Sprintf("\\u%0.4X", intRr))
			} else {
				b.WriteRune(rr)
			}
		}
	}
	return b.String()
}

// Encodes a string to a TOML-compliant string value
func encodeTomlString(value string) string {
	var b bytes.Buffer

	for _, rr := range value {
		switch rr {
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			intRr := uint16(rr)
			if intRr < 0x001F {
				b.WriteString(fmt.Sprintf("\\u%0.4X", intRr))
			} else {
				b.WriteRune(rr)
			}
		}
	}
	return b.String()
}

func tomlValueStringRepresentation(v interface{}, indent string, arraysOneElementPerLine bool) (string, error) {
	// this interface check is added to dereference the change made in the writeTo function.
	// That change was made to allow this function to see formatting options.
	tv, ok := v.(*tomlValue)
	if ok {
		v = tv.value
	} else {
		tv = &tomlValue{}
	}

	switch value := v.(type) {
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case float64:
		// Ensure a round float does contain a decimal point. Otherwise feeding
		// the output back to the parser would convert to an integer.
		if math.Trunc(value) == value {
			return strings.ToLower(strconv.FormatFloat(value, 'f', 1, 32)), nil
		}
		return strings.ToLower(strconv.FormatFloat(value, 'f', -1, 32)), nil
	case string:
		if tv.multiline {
			return "\"\"\"\n" + encodeMultilineTomlString(value) + "\"\"\"", nil
		}
		return "\"" + encodeTomlString(value) + "\"", nil
	case []byte:
		b, _ := v.([]byte)
		return tomlValueStringRepresentation(string(b), indent, arraysOneElementPerLine)
	case bool:
		if value {
			return "true", nil
		}
		return "false", nil
	case time.Time:
		return value.Format(time.RFC3339), nil
	case nil:
		return "", nil
	}

	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Slice {
		var values []string
		for i := 0; i < rv.Len(); i++ {
			item := rv.Index(i).Interface()
			itemRepr, err := tomlValueStringRepresentation(item, indent, arraysOneElementPerLine)
			if err != nil {
				return "", err
			}
			values = append(values, itemRepr)
		}
		if arraysOneElementPerLine && len(values) > 1 {
			stringBuffer := bytes.Buffer{}
			valueIndent := indent + `  ` // TODO: move that to a shared encoder state

			stringBuffer.WriteString("[\n")

			for _, value := range values {
				stringBuffer.WriteString(valueIndent)
				stringBuffer.WriteString(value)
				stringBuffer.WriteString(`,`)
				stringBuffer.WriteString("\n")
			}

			stringBuffer.WriteString(indent + "]")

			return stringBuffer.String(), nil
		}
		return "[" + strings.Join(values, ",") + "]", nil
	}
	return "", fmt.Errorf("unsupported value type %T: %v", v, v)
}

func getTreeArrayLine(trees []*Tree) (line int) {
	// get lowest line number that is not 0
	for _, tv := range trees {
		if tv.position.Line < line || line == 0 {
			line = tv.position.Line
		}
	}
	return
}

func sortByLines(t *Tree) (vals []sortNode) {
	var (
		line  int
		lines []int
		tv    *Tree
		tom   *tomlValue
		node  sortNode
	)
	vals = make([]sortNode, 0)
	m := make(map[int]sortNode)

	for k := range t.values {
		v := t.values[k]
		switch v.(type) {
		case *Tree:
			tv = v.(*Tree)
			line = tv.position.Line
			node = sortNode{key: k, complexity: valueComplex}
		case []*Tree:
			line = getTreeArrayLine(v.([]*Tree))
			node = sortNode{key: k, complexity: valueComplex}
		default:
			tom = v.(*tomlValue)
			line = tom.position.Line
			node = sortNode{key: k, complexity: valueSimple}
		}
		lines = append(lines, line)
		vals = append(vals, node)
		m[line] = node
	}
	sort.Ints(lines)

	for i, line := range lines {
		vals[i] = m[line]
	}

	return vals
}

func sortAlphabetical(t *Tree) (vals []sortNode) {
	var (
		node     sortNode
		simpVals []string
		compVals []string
	)
	vals = make([]sortNode, 0)
	m := make(map[string]sortNode)

	for k := range t.values {
		v := t.values[k]
		switch v.(type) {
		case *Tree, []*Tree:
			node = sortNode{key: k, complexity: valueComplex}
			compVals = append(compVals, node.key)
		default:
			node = sortNode{key: k, complexity: valueSimple}
			simpVals = append(simpVals, node.key)
		}
		vals = append(vals, node)
		m[node.key] = node
	}

	// Simples first to match previous implementation
	sort.Strings(simpVals)
	i := 0
	for _, key := range simpVals {
		vals[i] = m[key]
		i++
	}

	sort.Strings(compVals)
	for _, key := range compVals {
		vals[i] = m[key]
		i++
	}

	return vals
}

func (t *Tree) writeTo(w io.Writer, indent, keyspace string, bytesCount int64, arraysOneElementPerLine bool) (int64, error) {
	return t.writeToOrdered(w, indent, keyspace, bytesCount, arraysOneElementPerLine, OrderAlphabetical)
}

func (t *Tree) writeToOrdered(w io.Writer, indent, keyspace string, bytesCount int64, arraysOneElementPerLine bool, ord marshalOrder) (int64, error) {
	var orderedVals []sortNode

	switch ord {
	case OrderPreserve:
		orderedVals = sortByLines(t)
	default:
		orderedVals = sortAlphabetical(t)
	}

	for _, node := range orderedVals {
		switch node.complexity {
		case valueComplex:
			k := node.key
			v := t.values[k]

			combinedKey := k
			if keyspace != "" {
				combinedKey = keyspace + "." + combinedKey
			}
			var commented string
			if t.commented {
				commented = "# "
			}

			switch node := v.(type) {
			// node has to be of those two types given how keys are sorted above
			case *Tree:
				tv, ok := t.values[k].(*Tree)
				if !ok {
					return bytesCount, fmt.Errorf("invalid value type at %s: %T", k, t.values[k])
				}
				if tv.comment != "" {
					comment := strings.Replace(tv.comment, "\n", "\n"+indent+"#", -1)
					start := "# "
					if strings.HasPrefix(comment, "#") {
						start = ""
					}
					writtenBytesCountComment, errc := writeStrings(w, "\n", indent, start, comment)
					bytesCount += int64(writtenBytesCountComment)
					if errc != nil {
						return bytesCount, errc
					}
				}
				writtenBytesCount, err := writeStrings(w, "\n", indent, commented, "[", combinedKey, "]\n")
				bytesCount += int64(writtenBytesCount)
				if err != nil {
					return bytesCount, err
				}
				bytesCount, err = node.writeToOrdered(w, indent+"  ", combinedKey, bytesCount, arraysOneElementPerLine, ord)
				if err != nil {
					return bytesCount, err
				}
			case []*Tree:
				for _, subTree := range node {
					writtenBytesCount, err := writeStrings(w, "\n", indent, commented, "[[", combinedKey, "]]\n")
					bytesCount += int64(writtenBytesCount)
					if err != nil {
						return bytesCount, err
					}

					bytesCount, err = subTree.writeToOrdered(w, indent+"  ", combinedKey, bytesCount, arraysOneElementPerLine, ord)
					if err != nil {
						return bytesCount, err
					}
				}
			}
		default: // Simple
			k := node.key
			v, ok := t.values[k].(*tomlValue)
			if !ok {
				return bytesCount, fmt.Errorf("invalid value type at %s: %T", k, t.values[k])
			}

			repr, err := tomlValueStringRepresentation(v, indent, arraysOneElementPerLine)
			if err != nil {
				return bytesCount, err
			}

			if v.comment != "" {
				comment := strings.Replace(v.comment, "\n", "\n"+indent+"#", -1)
				start := "# "
				if strings.HasPrefix(comment, "#") {
					start = ""
				}
				writtenBytesCountComment, errc := writeStrings(w, "\n", indent, start, comment, "\n")
				bytesCount += int64(writtenBytesCountComment)
				if errc != nil {
					return bytesCount, errc
				}
			}

			var commented string
			if v.commented {
				commented = "# "
			}
			writtenBytesCount, err := writeStrings(w, indent, commented, k, " = ", repr, "\n")
			bytesCount += int64(writtenBytesCount)
			if err != nil {
				return bytesCount, err
			}
		}
	}

	return bytesCount, nil
}

func writeStrings(w io.Writer, s ...string) (int, error) {
	var n int
	for i := range s {
		b, err := io.WriteString(w, s[i])
		n += b
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// WriteTo encode the Tree as Toml and writes it to the writer w.
// Returns the number of bytes written in case of success, or an error if anything happened.
func (t *Tree) WriteTo(w io.Writer) (int64, error) {
	return t.writeTo(w, "", "", 0, false)
}

// ToTomlString generates a human-readable representation of the current tree.
// Output spans multiple lines, and is suitable for ingest by a TOML parser.
// If the conversion cannot be performed, ToString returns a non-nil error.
func (t *Tree) ToTomlString() (string, error) {
	var buf bytes.Buffer
	_, err := t.WriteTo(&buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// String generates a human-readable representation of the current tree.
// Alias of ToString. Present to implement the fmt.Stringer interface.
func (t *Tree) String() string {
	result, _ := t.ToTomlString()
	return result
}

// ToMap recursively generates a representation of the tree using Go built-in structures.
// The following types are used:
//
//	* bool
//	* float64
//	* int64
//	* string
//	* uint64
//	* time.Time
//	* map[string]interface{} (where interface{} is any of this list)
//	* []interface{} (where interface{} is any of this list)
func (t *Tree) ToMap() map[string]interface{} {
	result := map[string]interface{}{}

	for k, v := range t.values {
		switch node := v.(type) {
		case []*Tree:
			var array []interface{}
			for _, item := range node {
				array = append(array, item.ToMap())
			}
			result[k] = array
		case *Tree:
			result[k] = node.ToMap()
		case *tomlValue:
			result[k] = node.value
		}
	}
	return result
}
