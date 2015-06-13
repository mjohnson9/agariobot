package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/nightexcessive/agario"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/sdl_ttf"
)

const (
	gamemode = "ffa"
)

const (
	framesPerSecond = 60
	frameTime       = time.Second / framesPerSecond
)

func handleSDLEvents(c chan sdl.Event, quitChan chan struct{}) {
	/*sdl.AddEventWatchFunc(func(ev sdl.Event) bool {
		c <- ev
		return true
	})*/
	for {
		select {
		case _, ok := <-quitChan:
			if !ok {
				return
			}
		default:
		}
		ev := sdl.WaitEventTimeout(250)
		if ev == nil {
			continue
		}
		c <- ev
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

	sdlEvents := make(chan sdl.Event, 16)
	gameEvents := make(chan chan struct{})
	quitChan := make(chan struct{})

	go handleGameEvents(gameEvents, g)

	go func() {
		var lastTick uint32
		for {
			select {
			case event := <-sdlEvents:
				switch event.(type) {
				case *sdl.QuitEvent:
					log.Printf("SDL requested exit. Stopping input loop...")
					close(quitChan)
					return
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

	handleSDLEvents(sdlEvents, quitChan)
	log.Printf("Input loop stopped gracefully")
}

//var randomNames = []string{"poland", "usa", "china", "russia", "canada", "australia", "spain", "brazil", "germany", "ukraine", "france", "sweden", "hitler", "north korea", "south korea", "japan", "united kingdom", "earth", "greece", "latvia", "lithuania", "estonia", "finland", "norway", "cia", "maldivas", "austria", "nigeria", "reddit", "yaranaika", "confederate", "9gag", "indiana", "4chan", "italy", "bulgaria", "tumblr", "2ch.hk", "hong kong", "portugal", "jamaica", "german empire", "mexico", "sanik", "switzerland", "croatia", "chile", "indonesia", "bangladesh", "thailand", "iran", "iraq", "peru", "moon", "botswana", "bosnia", "netherlands", "european union", "taiwan", "pakistan", "hungary", "satanist", "qing dynasty", "matriarchy", "patriarchy", "feminism", "ireland", "texas", "facepunch", "prodota", "cambodia", "steam", "piccolo", "ea", "india", "kc", "denmark", "quebec", "ayy lmao", "sealand", "bait", "tsarist russia", "origin", "vinesauce", "stalin", "belgium", "luxembourg", "stussy", "prussia", "8ch", "argentina", "scotland", "sir", "romania", "belarus", "wojak", "doge", "nasa", "byzantium", "imperial japan", "french kingdom", "somalia", "turkey", "mars", "pokerface", "8", "irs", "receita federal", "nazi", "ussr"}

var randomNames = []string{"Derp", "DerpBot"}

func randomName() string {
	return randomNames[rand.Intn(len(randomNames))]
}

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	defer func() {
		if *memprofile != "" {
			f, err := os.Create(*memprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}
	}()

	log.Printf("Getting current location...")
	desiredLocation := make(chan string, 1)
	go func() {
		curLocation, recommendedServer, err := agario.GetCurrentLocation()
		if err != nil {
			panic(err)
		}

		desiredLocation <- recommendedServer

		if len(recommendedServer) == 0 {
			log.Printf("WARNING: could not find desired region for %s", curLocation)
			return
		}

		log.Printf("Got location: %s", curLocation)
		log.Printf("Recommended server: %s", recommendedServer)
	}()

	log.Printf("Getting region info...")
	info, err := agario.GetInfo()
	if err != nil {
		panic(err)
	}

	var c *agario.Connection

	regionName := <-desiredLocation
	for _, region := range info.Regions {
		if (regionName != "" && region.Region != regionName) || region.GameMode != gamemode {
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

	ttf.Init()

	run(g)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
	}
}
