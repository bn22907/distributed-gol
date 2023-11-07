package gol

import (
	"fmt"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

// worker is a function representing a concurrent worker in the Game of Life simulation.
// It calculates the next state of a specific portion of the world (slice) based on the given range.
func worker(id int, p Params, world [][]byte, result chan<- [][]byte, c distributorChannels, turn int) {
	startRow := id * (p.ImageHeight / p.Threads)
	endRow := (id + 1) * (p.ImageHeight / p.Threads)

	// Calculate the next state for the specified range and send it to the result channel
	newWorld := calculateNextState(world, startRow, endRow, c, turn)
	result <- newWorld
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

func distribute(p Params, c distributorChannels, world [][]uint8, turn int) {
	// Create a 2D slice to store the world.
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%d%s%d", p.ImageWidth, "x", p.ImageHeight)

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
}
