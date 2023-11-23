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

var wg sync.WaitGroup
var kill = make(chan bool)

type GOLWorker struct {
	World   [][]byte
	Turn    int
	Mu      sync.Mutex
	Quit    bool
	Workers []*rpc.Client
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

func (g *GOLWorker) EvolveWorld(req stubs.EvolveWorldRequest, res *stubs.EvolveResponse) (err error) {
	g.Quit = false
	g.World = req.World
	p := gol.Params{
		Turns:       req.Turn,
		Threads:     req.Threads,
		ImageWidth:  req.ImageWidth,
		ImageHeight: req.ImageHeight,
	}
	g.Turn = 0

	//set up client connection
	//global list of clients
	workerPorts := ReadFileLines("workers.txt")
	fmt.Println(workerPorts)
	for _, detail := range workerPorts {
		client, err := rpc.Dial("tcp", detail)
		if err == nil {
			g.Workers = append(g.Workers, client)
		}

	}
	fmt.Println(g.Workers)

	// TODO: Execute all turns of the Game of Life.
	// Run Game of Life simulation for the specified number of turns
	for g.Turn < p.Turns && g.Quit == false {
		g.Mu.Lock()

		var newWorld [][]byte
		threads := len(g.Workers)
		results := make([]chan [][]uint8, threads)
		for id, workerClient := range g.Workers {
			results[id] = make(chan [][]uint8)
			go worker(id, g.World, results[id], p, workerClient, threads)
		}
		for i := 0; i < threads; i++ {
			filteredMatrix := <-results[i]
			newWorld = append(newWorld, filteredMatrix...)
		}

		g.World = newWorld
		g.Turn++
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

	// Close the existing client connections
	for _, client := range g.Workers {
		client.Close()
	}
	g.Workers = nil

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

	rpc.Register(&GOLWorker{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Printf("Error starting listener: %s\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	rpc.Accept(listener)
}
