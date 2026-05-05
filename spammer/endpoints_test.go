package spammer

import "testing"

func TestResolveEndpointSelectionUsesExplicitRPC(t *testing.T) {
	resolved, err := ResolveEndpointSelection("127.0.0.1:9999", "custom", "", "")
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if resolved.RPCURL != "http://127.0.0.1:9999" {
		t.Fatalf("unexpected rpc url: %s", resolved.RPCURL)
	}
	if resolved.RPCLabel != "custom" {
		t.Fatalf("unexpected rpc label: %s", resolved.RPCLabel)
	}
}

func TestSelectExecutionNodeByELClient(t *testing.T) {
	node, err := selectExecutionNode([]ExecutionNode{{ELClient: "geth", RPC: "127.0.0.1:1"}, {ELClient: "reth", RPC: "127.0.0.1:2"}}, "reth")
	if err != nil {
		t.Fatalf("select node: %v", err)
	}
	if node.ELClient != "reth" {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func TestSelectExecutionNodeDefaultsToSingleFirstNodeWhenClientOmitted(t *testing.T) {
	nodes := []ExecutionNode{
		{ELClient: "geth", RPC: "127.0.0.1:1"},
		{ELClient: "reth", RPC: "127.0.0.1:2"},
	}
	node, err := selectExecutionNode(nodes, "")
	if err != nil {
		t.Fatalf("select node: %v", err)
	}
	if node != nodes[0] {
		t.Fatalf("expected first node to be selected when el-client is omitted, got %+v want %+v", node, nodes[0])
	}
}
