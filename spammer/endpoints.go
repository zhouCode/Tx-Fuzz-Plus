package spammer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultEndpointsPath = "~/ethpackage/endpoints.json"

type EndpointInventory struct {
	ExecutionNodes []ExecutionNode `json:"execution_nodes"`
}

type ExecutionNode struct {
	Index    int     `json:"index"`
	ELClient string  `json:"el_client"`
	RPC      string  `json:"rpc"`
	WS       *string `json:"ws"`
}

type ResolvedEndpoint struct {
	RPCURL   string
	RPCLabel string
	Source   string
}

func ResolveEndpointSelection(rawRPC, rawLabel, rawEndpointsPath, rawELClient string) (ResolvedEndpoint, error) {
	if strings.TrimSpace(rawRPC) != "" {
		label := strings.TrimSpace(rawLabel)
		if label == "" {
			label = strings.TrimSpace(rawELClient)
		}
		if label == "" {
			label = "explicit-rpc"
		}
		return ResolvedEndpoint{RPCURL: normalizeRPCURL(rawRPC), RPCLabel: label}, nil
	}
	path := rawEndpointsPath
	if strings.TrimSpace(path) == "" {
		path = defaultEndpointsPath
	}
	inv, source, err := loadEndpointInventory(path)
	if err != nil {
		return ResolvedEndpoint{}, err
	}
	selected, err := selectExecutionNode(inv.ExecutionNodes, rawELClient)
	if err != nil {
		return ResolvedEndpoint{}, err
	}
	label := strings.TrimSpace(rawLabel)
	if label == "" {
		label = selected.ELClient
	}
	return ResolvedEndpoint{RPCURL: normalizeRPCURL(selected.RPC), RPCLabel: label, Source: source}, nil
}

func loadEndpointInventory(path string) (EndpointInventory, string, error) {
	var inv EndpointInventory
	expanded, err := expandHome(path)
	if err != nil {
		return inv, "", err
	}
	blob, err := os.ReadFile(expanded)
	if err != nil {
		return inv, expanded, err
	}
	if err := json.Unmarshal(blob, &inv); err != nil {
		return inv, expanded, err
	}
	return inv, expanded, nil
}

func selectExecutionNode(nodes []ExecutionNode, wanted string) (ExecutionNode, error) {
	if len(nodes) == 0 {
		return ExecutionNode{}, errors.New("endpoints inventory has no execution_nodes entries")
	}
	if strings.TrimSpace(wanted) == "" {
		return nodes[0], nil
	}
	for _, node := range nodes {
		if strings.EqualFold(node.ELClient, wanted) {
			return node, nil
		}
	}
	return ExecutionNode{}, fmt.Errorf("execution node with el_client=%q not found", wanted)
}

func normalizeRPCURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.Contains(raw, "://") {
		return raw
	}
	return "http://" + raw
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
