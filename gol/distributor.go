package gol

import (
	"fmt"
	"time"
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

// aliveCellsTicker is a function used for handling different commands and events during the Game of Life execution.
// It monitors the ticker channel for specific time intervals and responds to certain keypress commands to modify the execution or send events.
func aliveCellsTicker(c distributorChannels, turn int, ticker *time.Ticker, world [][]byte, p Params) {
	select {
	// When the ticker ticks
	case <-ticker.C:
		// Send an AliveCellsCount event indicating the number of alive cells at the current turn
		c.events <- AliveCellsCount{turn, len(calculateAliveCells(world))}

	// Check for keypress events
	case command := <-c.keyPresses:
		// React based on the keypress command
		switch command {
		case 's': // 's' key is pressed
			// StateChange event to indicate execution and save a PGM image
			c.events <- StateChange{turn, Executing}
			savePGMImage(c, world, p) // Function to save the current state as a PGM image

		case 'q': // 'q' key is pressed
			// StateChange event to indicate quitting and save a PGM image
			c.events <- StateChange{turn, Quitting}
			savePGMImage(c, world, p) // Function to save the current state as a PGM image
			close(c.events)           // Close the events channel

		case 'p': // 'p' key is pressed
			// StateChange event to indicate pausing and print current turn processing status
			c.events <- StateChange{turn, Paused}
			fmt.Printf("Current turn %d being processed\n", turn)
			// Wait for another 'p' keypress to resume execution
			for {
				if <-c.keyPresses == 'p' {
					break
				}
			}
			// StateChange event to indicate execution after pausing
			c.events <- StateChange{turn, Executing}
		}

	default: // No events
	}
	// Send a TurnComplete event for the current turn
	c.events <- TurnComplete{CompletedTurns: turn}
}

// savePGMImage is a function responsible for saving the current state of the world as a PGM image.
// It sends the world data via the ioOutput channel for writing the PGM file.
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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// Create a 2D slice to represent the world by reading input from the ioInput channel
	world := make([][]uint8, p.ImageHeight)

	// Initialize variables for tracking turns and creating a ticker
	turn := 0
	distribute(p, c, world, turn)

	// Send FinalTurnComplete event and save the final image as a PGM file
	c.events <- FinalTurnComplete{turn, calculateAliveCells(world)}
	savePGMImage(c, world, p)

	// Ensure IO has finished any output before exiting
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Send a StateChange event to indicate quitting
	c.events <- StateChange{p.Turns, Quitting}

	// Concurrently work with multiple workers to update the world state
	resultCh := make(chan [][]byte, p.Threads)
	for i := 0; i < p.Threads; i++ {
		go worker(i, p, world, resultCh, c, turn)
	}

	// Merge the results from workers to update the overall world state
	for i := 0; i < p.Threads; i++ {
		newWorld := <-resultCh
		for y := range newWorld {
			for x := range newWorld[y] {
				world[y][x] = newWorld[y][x]
			}
		}
	}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
