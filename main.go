package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"runtime/pprof"
	"time"

	"github.com/ajhager/engi"
	"github.com/nightexcessive/agario"
)

const (
	framesPerSecond = 60
	frameTime       = time.Second / framesPerSecond
)

func handleGameEvents(c chan struct{}, g *agario.Game) {
	for {
		g.RunOnce(false)
		for g.RunOnce(true) {
		}
		c <- struct{}{}
	}
}

func run(ig *agario.Game) {
	gameEvents := make(chan struct{})
	quitChan := make(chan struct{})

	g := &Game{
		g:        ig,
		quitChan: quitChan,
	}
	/*ai := &AI{
		g: ig,
	}*/
	ka := &keepAlive{
		g: ig,
	}

	go handleGameEvents(gameEvents, ig)
	go engi.Open("agariobot", 1280, 800, false, g)

	lastTick := time.Now()

mainLoop:
	for {
		select {
		case _, ok := <-quitChan:
			if !ok {
				break mainLoop
			}
		case <-gameEvents:
			dt := time.Now().Sub(lastTick)

			ig.Lock()

			ka.Update(dt)
			//ai.Update(dt)

			ig.Unlock()

			lastTick = time.Now()
		}
	}

	log.Printf("Gracefully stopped")
}

var randomNames = []string{"Derp", "Derp", "Derp", "Derp", "Derp", "Earth", "CIA", "Confederate", "Sanik", "Moon", "Qing Dynasty", "Matriarchy", "Patriarchy", "Feminism", "Steam", "Bait", "Vinesauce", "Sir", "Wojak", "Doge", "NASA", "Mars", "Pokerface", "8", "IRS"}

func randomName() string {
	return randomNames[rand.Intn(len(randomNames))]
}

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")

	gamemode = flag.String("gamemode", "ffa", "agar.io gamemode")
	region   = flag.String("region", "", "agar.io region (blank = closest)")
)

func main() {
	log.SetFlags(log.Lshortfile)

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
	if *region == "" {
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
	} else {
		desiredLocation <- *region
	}

	log.Printf("Getting region info...")
	info, err := agario.GetInfo()
	if err != nil {
		panic(err)
	}

	var c *agario.Connection

	regionName := <-desiredLocation
	for _, region := range info.Regions {
		if (regionName != "" && region.Region != regionName) || region.GameMode != *gamemode {
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

	//defer c.Close()

	g := agario.NewGame(c)
	//defer g.Close()

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
