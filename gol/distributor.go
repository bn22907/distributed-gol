package gol

import (
	"fmt"
	"log"
	"net/rpc"
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
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

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
				c.events <- CellFlipped{0, util.Cell{j, i}}
			}
		}
	}

	turn := 0
	// Connect to the server via RPC
	client, err := rpc.Dial("tcp", "127.0.0.1:8030") // Replace "127.0.0.1:8030" with your server's IP and port
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}

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

	live := true
	//go func() {
	//	for live {
	//		if !live {
	//			break
	//		}
	//		empty := stubs.Empty{}
	//		cellFlippedResponse := &stubs.GetBrokerCellFlippedResponse{}
	//
	//		err = client.Call(stubs.GetBrokerCellFlippedHandler, empty, cellFlippedResponse)
	//		cellUpdates := cellFlippedResponse.Cell
	//		fmt.Println(cellFlippedResponse.Turn)
	//		if len(cellUpdates) != 0 && live {
	//			for i := range cellUpdates {
	//				c.events <- CellFlipped{cellFlippedResponse.Turn, cellUpdates[i]}
	//			}
	//		}
	//		time.Sleep(5 * time.Millisecond)
	//	}
	//}()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			empty := stubs.Empty{}

			go func() {
				for live {
					if !live {
						break
					}
					empty := stubs.Empty{}
					cellFlippedResponse := &stubs.GetBrokerCellFlippedResponse{}

					err = client.Call(stubs.GetBrokerCellFlippedHandler, empty, cellFlippedResponse)
					cellUpdates := cellFlippedResponse.Cell
					fmt.Println(cellFlippedResponse.Turn)
					if len(cellUpdates) != 0 && live {
						for i := range cellUpdates {
							c.events <- CellFlipped{cellFlippedResponse.Turn, cellUpdates[i]}
						}
					}
					time.Sleep(5 * time.Millisecond)
				}
			}()

			getTurnDoneRes := &stubs.GetTurnDoneResponse{}
			client.Call(stubs.GetTurnDoneHandler, empty, getTurnDoneRes)
			if getTurnDoneRes.TurnDone {
				c.events <- TurnComplete{CompletedTurns: getTurnDoneRes.Turn}
			}

			select {
			case <-ticker.C:
				aliveCellsCountResponse := &stubs.AliveCellsCountResponse{}

				err = client.Call(stubs.AliveCellsCountHandler, empty, aliveCellsCountResponse)
				if err != nil {
					log.Fatal("call error : ", err)
					return
				}
				numberAliveCells := aliveCellsCountResponse.AliveCellsCount
				turn := aliveCellsCountResponse.CompletedTurns

				c.events <- AliveCellsCount{turn, numberAliveCells}
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
				world = getGlobal.World
				turn = getGlobal.Turns

				switch command {
				case 's': // 's' key is pressed
					// StateChange event to indicate execution and save a PGM image
					c.events <- StateChange{turn, Executing}
					savePGMImage(c, world, p) // Function to save the current state as a PGM image

				case 'q': // 'q' key is pressed
					// StateChange event to indicate quitting and save a PGM image
					err = client.Call(stubs.QuitHandler, empty, emptyResponse)
					c.events <- StateChange{turn, Quitting}
					savePGMImage(c, world, p) // Function to save the current state as a PGM image
					close(c.events)           // Close the events channel
					live = false

				case 'k':
					err = client.Call(stubs.KillServerHandler, empty, emptyResponse)
					c.events <- StateChange{turn, Quitting}
					savePGMImage(c, world, p) // Function to save the current state as a PGM image
					live = false
					close(c.events) // Close the events channel

				case 'p': // 'p' key is pressed
					c.events <- StateChange{turn, Paused}
					err = client.Call(stubs.PauseHandler, empty, emptyResponse)
					fmt.Printf("Current turn %d being processed\n", turn)
					for {
						if <-c.keyPresses == 'p' {
							err = client.Call(stubs.UnpauseHandler, empty, emptyResponse)
							break
						}
					}
					// StateChange event to indicate execution after pausing
					c.events <- StateChange{turn, Executing}
				}
			default: // No events
				if turn == p.Turns {
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
}

func savePGMImage(c distributorChannels, world [][]byte, p Params) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, p.Turns)

	// Iterate over the world and send each cell's value to the ioOutput channel for writing the PGM image
	for i := range world {
		for j := range world[i] {
			c.ioOutput <- world[i][j] // Send the current cell value to the output channel
		}
	}
}
