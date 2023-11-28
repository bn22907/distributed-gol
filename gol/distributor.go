package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
	mu         sync.Mutex
}

type race struct {
	turn   int
	client *rpc.Client
	mu     sync.Mutex
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c *distributorChannels) {

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

	// Connect to the server via RPC
	client, err := rpc.Dial("tcp", "127.0.0.1:8030") // Replace "127.0.0.1:8030" with your server's IP and port
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}

	empty := stubs.Empty{}
	continueResponse := &stubs.GetContinueResponse{}
	err = client.Call(stubs.GetContinueHandler, empty, continueResponse)

	if continueResponse.Continue {
		world = continueResponse.World
		fmt.Printf("Continuing From Turn %d\n", continueResponse.Turn)
	}

	// Send CellFlipped events for any initial live cells in the world.
	for i := range world {
		for j := range world[i] {
			if world[i][j] == 255 {
				c.events <- CellFlipped{0, util.Cell{j, i}}
			}
		}
	}

	var turn int

	r := race{turn: turn, client: client}

	//request to make to server for evolving the world
	evolveRequest := stubs.EvolveWorldRequest{
		World:       world,
		Width:       p.ImageWidth,
		Height:      p.ImageHeight,
		Turn:        p.Turns,
		Threads:     p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
	evolveResponse := &stubs.EvolveResponse{}

	//live := true
	goWorld := world
	done := false
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		tickSDL := time.NewTicker(1 * time.Millisecond)
		goDone := done
		defer ticker.Stop()
		defer tickSDL.Stop()
		for {
			empty := stubs.Empty{}
			if goDone {
				return
			}
			select {
			case <-tickSDL.C:
				c.mu.Lock()
				cellFlippedResponse := &stubs.GetBrokerCellFlippedResponse{}
				err = client.Call(stubs.GetBrokerCellFlippedHandler, empty, cellFlippedResponse)
				cellUpdates := cellFlippedResponse.FlippedEvents
				if len(cellUpdates) != 0 {
					for i := range cellUpdates {
						if !done {
							c.events <- CellFlipped{cellUpdates[i].CompletedTurns, cellUpdates[i].Cell}
						}
					}
					if !done {
						c.events <- TurnComplete{CompletedTurns: cellUpdates[0].CompletedTurns}
					}
				}
				c.mu.Unlock()

			case <-ticker.C:
				c.mu.Lock()
				aliveCellsCountResponse := &stubs.AliveCellsCountResponse{}
				err = client.Call(stubs.AliveCellsCountHandler, empty, aliveCellsCountResponse)
				if err != nil {
					log.Fatal("call error : ", err)
					return
				}
				numberAliveCells := aliveCellsCountResponse.AliveCellsCount
				r.turn = aliveCellsCountResponse.CompletedTurns
				if !done {
					c.events <- AliveCellsCount{r.turn, numberAliveCells}
				}
				c.mu.Unlock()
				// Check for keypress events
			case command := <-c.keyPresses:
				// React based on the keypress command
				empty := stubs.Empty{}
				emptyResponse := &stubs.Empty{}
				getGlobal := &stubs.GetGlobalResponse{}
				err = client.Call(stubs.GetGlobalHandler, empty, getGlobal)
				if err != nil {
					log.Fatal("call error : ", err)
					return
				}
				goWorld = getGlobal.World
				r.turn = getGlobal.Turns

				switch command {
				case 's': // 's' key is pressed
					// StateChange event to indicate execution and save a PGM image
					c.mu.Lock()
					c.events <- StateChange{r.turn, Executing}
					c.mu.Unlock()
					savePGMImage(c, goWorld, p) // Function to save the current state as a PGM image

				case 'q': // 'q' key is pressed
					// StateChange event to indicate quitting and save a PGM image
					err = client.Call(stubs.QuitHandler, empty, emptyResponse)
					c.mu.Lock()
					c.events <- StateChange{r.turn, Quitting}
					c.mu.Unlock()
					savePGMImage(c, goWorld, p) // Function to save the current state as a PGM image
					close(c.events)             // Close the events channel
					done = true
					return
					//live = false

				case 'k':
					err = client.Call(stubs.KillServerHandler, empty, emptyResponse)
					c.mu.Lock()
					c.events <- StateChange{r.turn, Quitting}
					c.mu.Unlock()
					savePGMImage(c, goWorld, p) // Function to save the current state as a PGM image
					//live = false
					close(c.events) // Close the events channel
					done = true
					return

				case 'p': // 'p' key is pressed
					c.events <- StateChange{r.turn, Paused}
					err = client.Call(stubs.PauseHandler, empty, emptyResponse)
					fmt.Printf("Current turn %d being processed\n", turn)
					for {
						if <-c.keyPresses == 'p' {
							err = client.Call(stubs.UnpauseHandler, empty, emptyResponse)
							break
						}
					}
					// StateChange event to indicate execution after pausing
					c.events <- StateChange{r.turn, Executing}
				}
			default: // No events
				if r.turn == p.Turns {
					return
				}
			}
		}
	}()
	err = client.Call(stubs.EvolveWorldHandler, evolveRequest, evolveResponse)
	if err != nil {
		log.Fatal("call error : ", err)
	}
	world = evolveResponse.World
	turn = evolveResponse.Turn

	aliveCellsRequest := stubs.CalculateAliveCellsRequest{
		World: world,
	}
	aliveCellsResponse := &stubs.CalculateAliveCellsResponse{}

	err = client.Call(stubs.AliveCellsHandler, aliveCellsRequest, aliveCellsResponse)
	if err != nil {
		log.Fatal("call error : ", err)
	}
	aliveCells := aliveCellsResponse.AliveCells

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{turn, aliveCells}
	savePGMImage(c, world, p)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
	done = true

}

func savePGMImage(c *distributorChannels, world [][]byte, p Params) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, p.Turns)
	// Iterate over the world and send each cell's value to the ioOutput channel for writing the PGM image
	for i := range world {
		for j := range world[i] {
			c.ioOutput <- world[i][j] // Send the current cell value to the output channel
		}
	}
}
