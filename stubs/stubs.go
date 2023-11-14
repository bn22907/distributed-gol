package stubs

import "uk.ac.bris.cs/gameoflife/util"

var EvolveWorldHandler = "GOLWorker.EvolveWorld"

type EvolveResponse struct {
	World [][]byte
	Turn  int
}

// EvolveWorldRequest request struct that holds the parameters for the RPC call
type EvolveWorldRequest struct {
	World       [][]byte
	Width       int
	Height      int
	Turn        int
	Threads     int
	ImageHeight int
	ImageWidth  int
}

var AliveCellsHandler = "GOLWorker.CalculateAliveCells"

// CalculateAliveCellsRequest request struct that holds parameters
type CalculateAliveCellsRequest struct {
	World [][]byte
}
type CalculateAliveCellsResponse struct {
	AliveCells []util.Cell
}

var AliveCellsCountHandler = "GOLWorker.AliveCellsCount"

type AliveCellsCountResponse struct {
	AliveCellsCount int
	CompletedTurns  int
}

type EmptyReq struct{}
