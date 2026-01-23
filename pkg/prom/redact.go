package prom

import (
	"github.com/pkg/errors"
	yamlv3 "gopkg.in/yaml.v3"
)

const redactedSecretValue = "<secret>"

// RedactHTTPHeadersSecrets replaces http_headers secrets with <secret> placeholders.
func RedactHTTPHeadersSecrets(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var node yamlv3.Node
	if err := yamlv3.Unmarshal(raw, &node); err != nil {
		return nil, errors.Wrap(err, "unmarshal config for http_headers redaction")
	}
	redactHTTPHeaders(&node)
	redacted, err := yamlv3.Marshal(&node)
	if err != nil {
		return nil, errors.Wrap(err, "marshal config for http_headers redaction")
	}
	return redacted, nil
}

func redactHTTPHeaders(node *yamlv3.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yamlv3.DocumentNode, yamlv3.SequenceNode:
		for _, child := range node.Content {
			redactHTTPHeaders(child)
		}
	case yamlv3.MappingNode:
		for i := 0; i < len(node.Content)-1; i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Value == "http_headers" {
				redactHTTPHeadersMap(value)
				continue
			}
			redactHTTPHeaders(value)
		}
	}
}

func redactHTTPHeadersMap(node *yamlv3.Node) {
	if node == nil || node.Kind != yamlv3.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		headerNode := node.Content[i+1]
		if headerNode.Kind != yamlv3.MappingNode {
			continue
		}
		secretsNode := findMapValue(headerNode, "secrets")
		if secretsNode == nil {
			continue
		}
		switch secretsNode.Kind {
		case yamlv3.SequenceNode:
			for _, secretNode := range secretsNode.Content {
				secretNode.Value = redactedSecretValue
			}
		case yamlv3.ScalarNode:
			secretsNode.Value = redactedSecretValue
		}
	}
}

func findMapValue(node *yamlv3.Node, key string) *yamlv3.Node {
	if node == nil || node.Kind != yamlv3.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
