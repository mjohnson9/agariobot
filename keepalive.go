package main

import (
	"log"
	"time"

	"github.com/nightexcessive/agario"
)

// keepAlive keeps the AI alive. If the AI dies, it tries to respawn the AI.
type keepAlive struct {
	g *agario.Game

	tryNum  int
	nextTry time.Time

	currentNickname string
}

func (k *keepAlive) Update(_ time.Duration) {
	if k.currentlyAlive() {
		if k.tryNum != 0 {
			log.Printf("Spawned as \"%s\"", k.currentNickname)

			k.tryNum = 0
			k.currentNickname = ""
			k.nextTry = time.Time{}
		}

		return
	}

	now := time.Now()
	if k.nextTry.After(now) {
		return
	}

	const (
		retryTime  = 300 * time.Millisecond
		randomness = retryTime / 10
	)

	k.trySpawn()
	k.tryNum++
	k.nextTry = now.Add(retryTime * time.Duration(k.tryNum))
}

func (k *keepAlive) trySpawn() {
	if k.currentNickname == "" {
		k.currentNickname = randomName()
	}

	log.Printf("Trying to spawn as \"%s\"", k.currentNickname)
	k.g.SendNickname(k.currentNickname)
}

func (k *keepAlive) currentlyAlive() bool {
	if len(k.g.MyIDs) == 0 {
		return false
	}

	for id := range k.g.MyIDs {
		_, ok := k.g.Cells[id]
		if ok {
			return true
		}
	}

	return false
}
