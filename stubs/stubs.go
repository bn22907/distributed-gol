package stubs

import "uk.ac.bris.cs/gameoflife/util"

var EvolveWorldHandler = "GOLWorker.EvolveWorld"
var AliveCellsCountHandler = "GOLWorker.AliveCellsCount"
var AliveCellsHandler = "GOLWorker.CalculateAliveCells"
var GetGlobalHandler = "GOLWorker.GetGlobal"
var PauseHandler = "GOLWorker.Pause"
var UnpauseHandler = "GOLWorker.Unpause"
var QuitHandler = "GOLWorker.QuitServer"

var KillServerHandler = "GOLWorker.KillServer"

type EvolveResponse struct {
	World [][]byte
	Turn  int
}

type EvolveWorldRequest struct {
	World       [][]byte
	Width       int
	Height      int
	Turn        int
	Threads     int
	ImageHeight int
	ImageWidth  int
}
type CalculateAliveCellsRequest struct {
	World [][]byte
}
type CalculateAliveCellsResponse struct {
	AliveCells []util.Cell
}
type AliveCellsCountResponse struct {
	AliveCellsCount int
	CompletedTurns  int
}
type GetGlobalResponse struct {
	World [][]byte
	Turns int
}
type Empty struct{}
