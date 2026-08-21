package game

import (
	"encoding/json"
	"os"
	"testing"
)

func TestBoardV1ContractMatchesRuntimeTopology(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/board.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Version string `json:"version"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	board := NewBoard()
	if contract.Version != board.Version || contract.Width != board.Width || contract.Height != board.Height {
		t.Fatalf("contract/runtime mismatch: %#v vs %s %dx%d", contract, board.Version, board.Width, board.Height)
	}
	if len(board.Nodes) != board.Width*board.Height || len(board.Edges) == 0 {
		t.Fatalf("invalid generated topology: nodes=%d edges=%d", len(board.Nodes), len(board.Edges))
	}
	for _, seat := range Seats {
		if got := len(board.DeploymentNodes(seat)); got != 25 {
			t.Fatalf("%s deployment has %d nodes", seat, got)
		}
	}
}
