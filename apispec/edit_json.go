package apispec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

func jsonIndent(text []byte) int {
	for line := range strings.SplitSeq(string(text), "\n") {
		if trimmed := strings.TrimLeft(line, " "); trimmed != "" && len(trimmed) < len(line) {
			return len(line) - len(trimmed)
		}
	}
	return defaultIndentUnit
}

func writeJSON(root *yaml.Node, indent int, trailingNewline bool) ([]byte, error) {
	var out bytes.Buffer
	if err := writeJSONNode(&out, root, strings.Repeat(" ", indent), 0); err != nil {
		return nil, err
	}
	if trailingNewline {
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

func writeJSONNode(out *bytes.Buffer, node *yaml.Node, unit string, depth int) error {
	switch node.Kind {
	case yaml.MappingNode:
		return writeJSONMapping(out, node, unit, depth)
	case yaml.SequenceNode:
		return writeJSONSequence(out, node, unit, depth)
	case yaml.AliasNode:
		return writeJSONNode(out, node.Alias, unit, depth)
	case yaml.ScalarNode:
		return writeJSONScalar(out, node)
	case yaml.DocumentNode:
		return errors.New("a document node inside a document")
	}
	return fmt.Errorf("unknown node kind %d", node.Kind)
}

func writeJSONMapping(out *bytes.Buffer, node *yaml.Node, unit string, depth int) error {
	if len(node.Content) == 0 {
		out.WriteString("{}")
		return nil
	}
	out.WriteString("{\n")
	for i := 0; i+1 < len(node.Content); i += nodesPerEntry {
		out.WriteString(strings.Repeat(unit, depth+1))
		if err := writeJSONString(out, node.Content[i].Value); err != nil {
			return err
		}
		out.WriteString(": ")
		if err := writeJSONNode(out, node.Content[i+1], unit, depth+1); err != nil {
			return err
		}
		if i+nodesPerEntry < len(node.Content) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString(strings.Repeat(unit, depth) + "}")
	return nil
}

func writeJSONSequence(out *bytes.Buffer, node *yaml.Node, unit string, depth int) error {
	if len(node.Content) == 0 {
		out.WriteString("[]")
		return nil
	}
	out.WriteString("[\n")
	for i, item := range node.Content {
		out.WriteString(strings.Repeat(unit, depth+1))
		if err := writeJSONNode(out, item, unit, depth+1); err != nil {
			return err
		}
		if i+1 < len(node.Content) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString(strings.Repeat(unit, depth) + "]")
	return nil
}

func writeJSONScalar(out *bytes.Buffer, node *yaml.Node) error {
	switch node.ShortTag() {
	case tagInt, tagFloat, tagBool:
		out.WriteString(node.Value)
	case "!!null":
		out.WriteString("null")
	default:
		return writeJSONString(out, node.Value)
	}
	return nil
}

func writeJSONString(out *bytes.Buffer, value string) error {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode a JSON string: %w", err)
	}
	out.Write(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	return nil
}
