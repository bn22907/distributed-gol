package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type DistributorChannels struct {
	Events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c DistributorChannels) {

	// Connect to the server via RPC
	client, err := rpc.Dial("tcp", "127.0.0.1:8030") // Replace "127.0.0.1:8030" with your server's IP and port
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%d%s%d", p.ImageWidth, "x", p.ImageHeight)

	// TODO: Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
		}
	}

	// Send CellFlipped events for any initial live cells in the world.
	for i := range world {
		for j := range world[i] {
			if world[i][j] == 255 {
				c.Events <- CellFlipped{0, util.Cell{j, i}}
			}
		}
	}
	turn := 0

	// golWorker := new(engine.GOLWorker)
	//request to make to server for evolving the world
	evolveRequest := stubs.EvolveWorldRequest{
		World:  world,
		Width:  p.ImageWidth,
		Height: p.ImageHeight,
		Turn:   p.Turns,
	}
	evolveResponse := &stubs.EvolveResponse{}
	// Make the RPC call
	//fmt.Println("call")

	err = client.Call(stubs.EvolveWorldHandler, evolveRequest, evolveResponse)
	if err != nil {
		log.Fatal("call error : ", err)
	}

	//fmt.Println("call oaishdfiaobdsf")

	world = evolveResponse.World
	turn = evolveResponse.Turn

	aliveCellsRequest := stubs.CalculateAliveCellsRequest{
		World: world,
	}
	aliveCellsResponse := &stubs.CalculateAliveCellsResponse{}

	client.Call(stubs.AliveCellsHandler, aliveCellsRequest, aliveCellsResponse)
	aliveCells := aliveCellsResponse.AliveCells

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.Events <- FinalTurnComplete{turn, aliveCells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.Events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.Events)
}
