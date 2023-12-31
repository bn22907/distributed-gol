package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"uk.ac.bris.cs/gameoflife/stubs"
)

var kill = make(chan bool)

type WorldOps struct {
}

func (w *WorldOps) CalculateWorld(req *stubs.WorldReq, res *stubs.WorldRes) (err error) {
	res.World = calculateNextState(req.World, req.Width, req.Height, req.StartRow, req.EndRow)
	return
}

func (w *WorldOps) KillWorker(req *stubs.Empty, res *stubs.Empty) (err error) {
	kill <- true
	return
}

func calculateNextState(world [][]byte, width int, height int, startRow int, endRow int) [][]byte {
	nextState := make([][]byte, endRow-startRow)

	for i := 0; i < endRow-startRow; i++ {
		nextState[i] = make([]byte, width)
	}

	for i := startRow; i < endRow; i++ {
		for j := 0; j < width; j++ {
			//sum of neighboring cells around the current one
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
				//if less than 2 neighbors then die
				if sum < 2 {
					nextState[i-startRow][j] = 0
				} else if sum == 2 || sum == 3 { //if 2 or 3 neighbors then unaffected
					nextState[i-startRow][j] = 255
				} else { //if more than 3 neighbors then die
					nextState[i-startRow][j] = 0
				}
			} else { //if dead cell
				//if 3 neighbors then alive
				if sum == 3 {
					nextState[i-startRow][j] = 255
				} else { //else unaffected
					nextState[i-startRow][j] = 0
				}
			}
		}
	}

	return nextState
}

func main() {
	pAddr := flag.String("port", "8040", "Port to listen on")
	flag.Parse()

	ops := &WorldOps{}
	rpc.Register(ops)

	go func() {
		for {
			if <-kill {
				os.Exit(1)
			}
		}
	}()

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer listener.Close()
	fmt.Println("Listening on port", *pAddr)
	rpc.Accept(listener)
}
