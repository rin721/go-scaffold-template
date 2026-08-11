package config

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func encodeDefaultDocument(object Object, format Format) ([]byte, error) {
	if err := validateObject(object); err != nil {
		return nil, err
	}
	switch format {
	case FormatYAML:
		return encodeYAML(object)
	case FormatJSON:
		return encodeJSON(object)
	default:
		return nil, fmt.Errorf("unsupported default configuration format %q", format)
	}
}

func encodeYAML(object Object) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{yamlObjectNode(object)}}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("encode YAML default configuration: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close YAML default configuration encoder: %w", err)
	}
	return normalizeFinalNewline(output.Bytes()), nil
}

func yamlObjectNode(object Object) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, field := range object {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: field.Name},
			yamlValueNode(field.Value.(configValue)),
		)
	}
	return node
}

func yamlValueNode(value configValue) *yaml.Node {
	switch value.kind {
	case valueString, valueDuration:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value.text}
	case valueBool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value.boolean)}
	case valueNumber:
		tag := "!!int"
		if strings.ContainsAny(value.text, ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value.text}
	case valueNull:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	case valueObject:
		return yamlObjectNode(value.object)
	case valueList:
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, element := range value.elements {
			node.Content = append(node.Content, yamlValueNode(element.(configValue)))
		}
		return node
	default:
		panic("validated default configuration contains an unknown value")
	}
}

func encodeJSON(object Object) ([]byte, error) {
	var output bytes.Buffer
	writeJSONObject(&output, object, 0)
	output.WriteByte('\n')
	return output.Bytes(), nil
}

func writeJSONObject(output *bytes.Buffer, object Object, depth int) {
	output.WriteByte('{')
	if len(object) > 0 {
		output.WriteByte('\n')
	}
	for index, field := range object {
		writeIndent(output, depth+1)
		output.WriteString(strconv.Quote(field.Name))
		output.WriteString(": ")
		writeJSONValue(output, field.Value.(configValue), depth+1)
		if index < len(object)-1 {
			output.WriteByte(',')
		}
		output.WriteByte('\n')
	}
	if len(object) > 0 {
		writeIndent(output, depth)
	}
	output.WriteByte('}')
}

func writeJSONValue(output *bytes.Buffer, value configValue, depth int) {
	switch value.kind {
	case valueString, valueDuration:
		output.WriteString(strconv.Quote(value.text))
	case valueBool:
		output.WriteString(strconv.FormatBool(value.boolean))
	case valueNumber:
		output.WriteString(value.text)
	case valueNull:
		output.WriteString("null")
	case valueObject:
		writeJSONObject(output, value.object, depth)
	case valueList:
		output.WriteByte('[')
		if len(value.elements) > 0 {
			output.WriteByte('\n')
		}
		for index, element := range value.elements {
			writeIndent(output, depth+1)
			writeJSONValue(output, element.(configValue), depth+1)
			if index < len(value.elements)-1 {
				output.WriteByte(',')
			}
			output.WriteByte('\n')
		}
		if len(value.elements) > 0 {
			writeIndent(output, depth)
		}
		output.WriteByte(']')
	default:
		panic("validated default configuration contains an unknown value")
	}
}

func writeIndent(output *bytes.Buffer, depth int) {
	output.Write(bytes.Repeat([]byte("  "), depth))
}

func normalizeFinalNewline(payload []byte) []byte {
	return append(bytes.TrimRight(payload, "\r\n"), '\n')
}
