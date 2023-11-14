package main

import (
	"flag"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GOLWorker struct {
	World [][]byte
	Turn  int
	Mu    sync.Mutex
}

func (g *GOLWorker) EvolveWorld(req stubs.EvolveWorldRequest, res *stubs.EvolveResponse) (err error) {
	g.World = req.World
	p := gol.Params{
		Turns:       req.Turn,
		Threads:     req.Threads,
		ImageWidth:  req.ImageWidth,
		ImageHeight: req.ImageHeight,
	}

	//turn := 0
	g.Turn = 0
	// TODO: Execute all turns of the Game of Life.
	// Run Game of Life simulation for the specified number of turns
	for g.Turn < p.Turns {
		g.Mu.Lock()
		g.World = calculateNextState(g.World, p.ImageWidth, p.ImageHeight, g.Turn) //this is making world empty
		//turn++
		g.Turn++
		g.Mu.Unlock()
	}

	res.World = g.World
	//res.Turn = turn
	res.Turn = g.Turn
	return
}

func calculateNextState(world [][]byte, width int, height int, turn int) [][]byte {

	nextState := make([][]byte, height)
	//2D slice of bytes, height is length of outer slice
	//each inner slice is of type byte

	//initialise each row of nextState with a slice of bytes of length width
	for i := range nextState {
		nextState[i] = make([]byte, width)
	}

	for i := 0; i < height; i++ {
		for j := 0; j < width; j++ {

			//sum of neighbouring cells around current one
			sum := (int(world[(i+height-1)%height][(j+width-1)%width]) +
				int(world[(i+height-1)%height][(j+width)%width]) +
				int(world[(i+height-1)%height][(j+width+1)%width]) +
				int(world[(i+height)%height][(j+width-1)%width]) +
				int(world[(i+height)%height][(j+width+1)%width]) +
				int(world[(i+height+1)%height][(j+width-1)%width]) +
				int(world[(i+height+1)%height][(j+width)%width]) +
				int(world[(i+height+1)%height][(j+width+1)%width])) / 255

			//if live cell
			if world[i][j] == 255 {

				//if less than 2 neighbours then die
				if sum < 2 {
					nextState[i][j] = 0
					//c.Events <- gol.CellFlipped{turn, util.Cell{j, i}}
				} else if sum == 2 || sum == 3 { //if 2 or 3 neighbours then unaffected
					nextState[i][j] = 255
				} else { //if more than 3 neighbours then  die
					nextState[i][j] = 0
					//c.Events <- gol.CellFlipped{turn, util.Cell{j, i}}
				}

				//if dead cell
			} else {

				//if 3 neighbours then alive
				if sum == 3 {
					nextState[i][j] = 255
					//c.Events <- gol.CellFlipped{turn, util.Cell{j, i}}

				} else { //else unaffected
					nextState[i][j] = 0
				}
			}
		}
	}
	return nextState
}

func (g *GOLWorker) CalculateAliveCells(req stubs.Empty, res *stubs.CalculateAliveCellsResponse) (err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	aliveCells := []util.Cell{}
	for i := range g.World { //height
		for j := range g.World[i] { //width
			if g.World[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{j, i})
			}
		}
	}
	res.AliveCells = aliveCells
	return
}

func (g *GOLWorker) AliveCellsCount(req stubs.Empty, res *stubs.AliveCellsCountResponse) (err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	aliveCells := []util.Cell{}
	for i := range g.World { //height
		for j := range g.World[i] { //width
			if g.World[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{j, i})
			}
		}
	}
	res.AliveCellsCount = len(aliveCells)
	res.CompletedTurns = g.Turn
	return
}

func (g *GOLWorker) GetGlobal(req stubs.Empty, res *stubs.GetGlobalResponse) (err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	res.World = g.World
	res.Turns = g.Turn
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to list on")
	flag.Parse()
	//World := make([][]byte)
	rpc.Register(&GOLWorker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
