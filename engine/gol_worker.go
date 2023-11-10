package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GOLWorker struct{}

func (g *GOLWorker) EvolveWorld(req stubs.EvolveWorldRequest, res *stubs.EvolveResponse) (err error) {

	fmt.Println("abc abc abc abc abc abc ")

	world := req.World
	p := gol.Params{
		Turns:   req.Turn,
		Threads: req.Threads,
	}

	turn := 0

	fmt.Println(p.Turns)
	// TODO: Execute all turns of the Game of Life.
	// Run Game of Life simulation for the specified number of turns
	for turn < p.Turns {
		fmt.Println(turn)
		world = calculateNextState(world, p.ImageWidth, p.ImageHeight, turn)
		turn++
	}

	res.World = world
	res.Turn = turn
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
	fmt.Println("reach")
	return nextState
}

func (g *GOLWorker) CalculateAliveCells(req stubs.CalculateAliveCellsRequest, res *stubs.CalculateAliveCellsResponse) (err error) {
	world := req.World

	aliveCells := []util.Cell{}
	for i := range world { //height
		for j := range world[i] { //width
			if world[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{j, i})
			}
		}
	}
	res.AliveCells = aliveCells
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to list on")
	flag.Parse()
	rpc.Register(&GOLWorker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
