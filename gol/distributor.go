package gol

import (
	"fmt"
	"time"
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

// worker is a function representing a concurrent worker in the Game of Life simulation.
// It calculates the next state of a specific portion of the world (slice) based on the given range.
func worker(id int, p Params, world [][]byte, result chan<- [][]byte, c distributorChannels, turn int) {
	startRow := id * (p.ImageHeight / p.Threads)
	endRow := (id + 1) * (p.ImageHeight / p.Threads)

	// Calculate the next state for the specified range and send it to the result channel
	newWorld := calculateNextState(world, startRow, endRow, c, turn)
	result <- newWorld
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
	// Create a 2D slice to store the world.
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%d%s%d", p.ImageWidth, "x", p.ImageHeight)

	// Create a 2D slice to represent the world by reading input from the ioInput channel
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
		}
	}

	// Send CellFlipped events for any initial live cells in the world
	for i := range world {
		for j := range world[i] {
			if world[i][j] == 255 {
				c.events <- CellFlipped{0, util.Cell{j, i}}
			}
		}
	}

	// Initialize variables for tracking turns and creating a ticker
	turn := 0
	ticker := time.NewTicker(2 * time.Second)

	// Run Game of Life simulation for the specified number of turns
	for turn < p.Turns {
		world = calculateNextState(world, p.ImageWidth, p.ImageHeight, c, turn)
		turn++
		// Send AliveCellsCount event every 2 seconds
		aliveCellsTicker(c, turn, ticker, world, p)
	}

	// Report the final state using FinalTurnCompleteEvent.
	aliveCells := make([]util.Cell, 0)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 { // Live cell
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}

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

func calculateNextState(world [][]byte, width int, height int, c distributorChannels, turn int) [][]byte {

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
					c.events <- CellFlipped{turn, util.Cell{j, i}}
				} else if sum == 2 || sum == 3 { //if 2 or 3 neighbours then unaffected
					nextState[i][j] = 255
				} else { //if more than 3 neighbours then  die
					nextState[i][j] = 0
					c.events <- CellFlipped{turn, util.Cell{j, i}}
				}

				//if dead cell
			} else {

				//if 3 neighbours then alive
				if sum == 3 {
					nextState[i][j] = 255
					c.events <- CellFlipped{turn, util.Cell{j, i}}

				} else { //else unaffected
					nextState[i][j] = 0
				}
			}
		}
	}

	return nextState
}

func calculateAliveCells(world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	for i := range world { //height
		for j := range world[i] { //width
			if world[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{j, i})
			}
		}
	}
	return aliveCells
}
