package main

import (
	"log"
	"os"
	"runtime"
	"time"

	"github.com/nightexcessive/agario"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	name = "DerpBot"

	regionName = "US-Atlanta"
	gamemode   = "ffa"
)

const (
	framesPerSecond = 60
	frameTime       = time.Second / framesPerSecond
)

func handleSDLEvents(c chan sdl.Event) {
	/*sdl.AddEventWatchFunc(func(ev sdl.Event) bool {
		c <- ev
		return true
	})*/
	for {
		c <- sdl.WaitEvent()
	}
}

func handleGameEvents(c chan chan struct{}, g *game) {
	respChan := make(chan struct{})
	for {
		g.Game.RunOnce(false)
		for g.Game.RunOnce(true) {
		}
		c <- respChan
		<-respChan // Wait for the render to finish so that we don't have to use mutexes
	}
}

func run(ig *agario.Game) {
	g := createGame(ig)

	sdlEvents := make(chan sdl.Event)
	gameEvents := make(chan chan struct{})

	go handleGameEvents(gameEvents, g)

	go func() {
		var lastTick uint32
		for {
			select {
			case event := <-sdlEvents:
				switch event.(type) {
				case *sdl.QuitEvent:
					os.Exit(0)
				case *sdl.MouseMotionEvent:
				default:
					log.Printf("SDL event: %T", event)
				}
			case respChan := <-gameEvents:
				dt := sdl.GetTicks() - lastTick
				shouldRun := g.Tick(time.Duration(dt) * time.Millisecond)
				lastTick = sdl.GetTicks()
				if !shouldRun {
					os.Exit(0)
				}
				respChan <- struct{}{}
			}
		}
	}()

	handleSDLEvents(sdlEvents)
}

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	log.Printf("Getting region info...")
	info, err := agario.GetInfo()
	if err != nil {
		panic(err)
	}

	var c *agario.Connection

	for _, region := range info.Regions {
		if region.Region != regionName || region.GameMode != gamemode {
			continue
		}

		log.Printf("Connecting to %s:%s...", region.Region, region.GameMode)

		c, err = region.Connect()
		if err != nil {
			panic(err)
		}

		log.Printf("Connected. Server IP: %s", c.Addr)
		break
	}
	if c == nil {
		log.Printf("Unable to find region %s with gamemode %s", regionName, gamemode)
		os.Exit(1)
	}

	defer c.Close()

	g := agario.NewGame(c)
	defer g.Close()

	log.Printf("Initializing SDL...")

	runtime.LockOSThread() // Lock this to the OS thread. We'll use this thread for rendering and event handling.
	sdl.Init(sdl.INIT_EVERYTHING)

	run(g)

	/*mainLoop:
	for {
		startTicks := sdl.GetTicks()
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				break mainLoop
			}
		}
		for g.RunOnce() {
		}

		if updated {
			gameHelper.Tick(g)
			updated = false
		}

		gameHelper.Redraw()

		frameMs := sdl.GetTicks() - startTicks

		log.Printf("frame took %dms (Time until next frame: %dms)", frameMs, frameTime-frameMs)

		frameSleep := int32(frameTime) - int32(frameMs)
		if frameSleep <= 0 {
			continue
		}
		sdl.Delay(uint32(frameSleep))
	}*/
}
