package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strings"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var kill = make(chan bool)

type GOLWorker struct {
	LastWorld     [][]byte
	World         [][]byte
	Turn          int
	Mu            sync.Mutex
	Quit          bool
	Workers       []*rpc.Client
	Cell          util.Cell
	TurnDone      bool
	CellUpdates   []util.Cell
	FlippedEvents []stubs.FlippedEvent
}

//reads worker addresses line by line
func ReadFileLines(filePath string) []string {

	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// Split the line into individual elements based on space
		elements := strings.Fields(line)
		lines = append(lines, elements...)
	}

	if err := scanner.Err(); err != nil {
		return nil
	}

	return lines
}

func worker(id int, world [][]byte, results chan<- [][]byte, p gol.Params, client *rpc.Client, threads int) {
	var heightDiff = float32(p.ImageHeight) / float32(threads)

	// Calculate StartRow and EndRow based on the thread ID
	startRow := int(float32(id) * heightDiff)
	endRow := int(float32(id+1) * heightDiff)

	// Ensure that EndRow does not exceed the total number of rows
	if endRow > p.ImageHeight {
		endRow = p.ImageHeight
	}

	worldReq := stubs.WorldReq{
		World:    world,
		StartRow: startRow,
		EndRow:   endRow,
		Width:    p.ImageWidth,
		Height:   p.ImageHeight,
	}

	//create a response
	worldRes := &stubs.WorldRes{
		World: [][]byte{},
	}

	err := client.Call(stubs.WorldHandler, worldReq, worldRes)
	if err != nil {
		print(err)
	}

	results <- worldRes.World
	return
}

func worldSize(world [][]byte) {
	nonEmptyCount := 0
	for _, row := range world {
		for _, cell := range row {
			if cell != 0 {
				nonEmptyCount++
			}
		}
	}
	fmt.Printf("Number of non-empty cells: %d\n", nonEmptyCount)
}

func (g *GOLWorker) EvolveWorld(req stubs.EvolveWorldRequest, res *stubs.EvolveResponse) (err error) {
	fmt.Println("evolve world called")
	g.Quit = false
	// Create a separate copy of the input world to work on
	g.World = make([][]byte, len(req.World))
	for i := range req.World {
		g.World[i] = make([]byte, len(req.World[i]))
		copy(g.World[i], req.World[i])
	}

	worldSize(g.World)

	g.LastWorld = g.World
	p := gol.Params{
		Turns:       req.Turn,
		Threads:     req.Threads,
		ImageWidth:  req.ImageWidth,
		ImageHeight: req.ImageHeight,
	}
	g.Turn = 0

	// TODO: Execute all turns of the Game of Life.
	// Run Game of Life simulation for the specified number of turns
	for g.Turn < p.Turns && g.Quit == false {
		g.Mu.Lock()

		var newWorld [][]byte
		threads := len(g.Workers)
		if g.Turn == 0 {
			fmt.Printf("threads: %d\n", threads)
		}
		results := make([]chan [][]uint8, threads)
		for id, workerClient := range g.Workers {
			results[id] = make(chan [][]uint8)
			go worker(id, g.World, results[id], p, workerClient, threads)
		}
		for i := 0; i < threads; i++ {
			slice := <-results[i]
			newWorld = append(newWorld, slice...)
		}

		g.World = newWorld
		g.Turn++
		g.TurnDone = true
		g.Mu.Unlock()
	}

	res.World = g.World
	res.Turn = g.Turn
	return
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
func (g *GOLWorker) QuitServer(req stubs.Empty, res *stubs.Empty) (err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	g.Quit = true
	empty := make([][]byte, len(g.World))
	g.World = empty
	g.Turn = 0
	g.CellUpdates = []util.Cell{}

	return
}
func (g *GOLWorker) Pause(req stubs.Empty, res *stubs.Empty) (err error) {
	g.Mu.Lock()
	return
}
func (g *GOLWorker) Unpause(req stubs.Empty, res *stubs.Empty) (err error) {
	g.Mu.Unlock()
	return
}
func (g *GOLWorker) KillServer(req stubs.Empty, res *stubs.Empty) (err error) {
	// Close the existing client connections
	emptyRes := stubs.Empty{}

	for _, client := range g.Workers {
		err = client.Call(stubs.KillHandler, req, emptyRes)
		client.Close()
	}

	g.Quit = true
	kill <- true
	return
}

func (g *GOLWorker) GetTurnDone(req stubs.Empty, res *stubs.GetTurnDoneResponse) (err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	res.TurnDone = g.TurnDone
	res.Turn = g.Turn
	g.TurnDone = false
	return
}

func (g *GOLWorker) GetCellFlipped(req stubs.Empty, res *stubs.GetBrokerCellFlippedResponse) (err error) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	g.FlippedEvents = []stubs.FlippedEvent{}
	for _, cell := range findFlippedCells(g.World, g.LastWorld) {
		flippedEvent := stubs.FlippedEvent{
			CompletedTurns: g.Turn,
			Cell:           cell,
		}
		g.FlippedEvents = append(g.FlippedEvents, flippedEvent)
	}

	g.LastWorld = g.World
	res.FlippedEvents = g.FlippedEvents
	return
}

func findFlippedCells(next [][]byte, current [][]byte) []util.Cell {
	var flipped []util.Cell

	if len(current) == 0 || len(next) == 0 || len(current[0]) == 0 || len(next[0]) == 0 {
		return flipped
	}

	xorWorld := xor2D(current, next)

	for i := 0; i < len(xorWorld); i++ {
		for j := 0; j < len(xorWorld[0]); j++ {
			if xorWorld[i][j] != 0 {
				flipped = append(flipped, util.Cell{j, i})
			}
		}
	}

	return flipped
}

// xor2D performs element-wise XOR on two [][]byte slices
func xor2D(a, b [][]byte) [][]byte {
	numRows := len(a)
	numCols := len(a[0])

	result := make([][]byte, numRows)
	for i := 0; i < numRows; i++ {
		result[i] = make([]byte, numCols)
		for j := 0; j < numCols; j++ {
			result[i][j] = a[i][j] ^ b[i][j]
		}
	}

	return result
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	go func() {
		for {
			if <-kill {
				os.Exit(1)
			}
		}
	}()

	var workers []*rpc.Client
	workerPorts := ReadFileLines("workers.txt")
	for _, detail := range workerPorts {
		client, err := rpc.Dial("tcp", detail)
		if err == nil {
			workers = append(workers, client)
		}
	}

	rpc.Register(&GOLWorker{Workers: workers})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Printf("Error starting listener: %s\n", err)
		os.Exit(1)
	}

	//set up client connection
	//global list of clients

	defer listener.Close()

	rpc.Accept(listener)
}
