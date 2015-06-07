package main

import (
	"math"
	"time"

	"github.com/nightexcessive/agario"
)

const (
	stateFleeing = iota
	stateHunting = iota
	stateFeeding = iota
	stateIdle    = iota
)

type AI struct {
	g *game

	State byte

	OwnCells []*agario.CellUpdate

	Predators []*agario.CellUpdate
	Prey      []*agario.CellUpdate
	Food      []*agario.CellUpdate
}

func (ai *AI) Update(dt time.Duration) {
	ai.updateOwnCells()

	me := ai.getSmallestCell()

	ai.Predators = make([]*agario.CellUpdate, 0)
	ai.Prey = make([]*agario.CellUpdate, 0)
	ai.Food = make([]*agario.CellUpdate, 0)

	ignoreSize := me.Size / 10
	if ignoreSize < 100 {
		ignoreSize = 100
	}

	predatorSize := int16(float64(me.Size)*1.25) - 1
	preySize := int16(float64(me.Size) / 1.25)

	for _, cell := range ai.g.Game.Cells {
		if _, myCell := ai.g.Game.MyIDs[cell.ID]; myCell {
			continue
		}

		switch {
		case cell.Size < 10: // Food. Players start at 10 and can't fall below it.
			ai.Food = append(ai.Food, cell)
		case cell.Size <= preySize:
			ai.Prey = append(ai.Prey, cell)
		case cell.Size >= predatorSize:
			ai.Predators = append(ai.Predators, cell)
		}
	}
}

func (ai *AI) Execute() {
	me := ai.getSmallestCell()

	if len(ai.Predators) > 0 {
		isFleeing := ai.flee(me)
		if isFleeing {
			ai.State = stateFleeing
			return
		}
	}

	if len(ai.Prey) > 0 {
		isHunting := ai.hunt(me)
		if isHunting {
			ai.State = stateHunting
			return
		}
	}

	if len(ai.Food) > 0 {
		isFeeding := ai.feed(me)
		if isFeeding {
			ai.State = stateFeeding
			return
		}
	}

	ai.g.Game.SetTargetPos(float64(me.Point.X), float64(me.Point.Y))
	ai.State = stateIdle
}

// flee moves directly away from the nearest cell that is capable of eating us
func (ai *AI) flee(me *agario.CellUpdate) bool {
	closestPredator := ai.getClosest(me.Point, ai.Predators)
	if closestPredator == nil {
		return false
	}

	// Find angle between us and the predator
	deltaX := float64(closestPredator.X - me.X)
	deltaY := float64(closestPredator.Y - me.Y)
	angle := math.Atan2(deltaX, deltaY)

	if angle > math.Pi {
		angle -= math.Pi
	} else {
		angle += math.Pi
	}

	// Position to move to
	targetX := float64(me.X) + (500 * math.Sin(angle))
	targetY := float64(me.Y) + (500 * math.Cos(angle))

	ai.g.Game.SetTargetPos(targetX, targetY)

	return true
}

// hunt attempts to kill the closest cell that can be killed by splitting
func (ai *AI) hunt(me *agario.CellUpdate) bool {
	if len(ai.OwnCells) > 1 {
		// We won't intentionally split into more than two
		return false
	}

	canSplitKillSize := int16(float64(me.Size) / 2 / 1.25)
	splitDistance := square(4*(40+(ai.getSpeed(me)*4)) + (float64(me.Size) * 1.75))

	closestPrey := ai.getClosestFiltered(me.Point, ai.Prey, func(cell *agario.CellUpdate) bool {
		return cell.Size <= canSplitKillSize && dist2(me.Point, cell.Point) <= splitDistance
	})
	if closestPrey == nil {
		return false
	}

	ai.g.Game.SetTargetPos(float64(closestPrey.Point.X), float64(closestPrey.Point.Y))
	ai.g.Game.Split()

	return true
}

// feed attempts to eat the nearest food cell
func (ai *AI) feed(me *agario.CellUpdate) bool {
	closestFood := ai.getClosest(me.Point, ai.Food)
	if closestFood == nil {
		return false
	}

	ai.g.Game.SetTargetPos(float64(closestFood.Point.X), float64(closestFood.Point.Y))

	return true
}

func (ai *AI) getClosest(p agario.Point, cells []*agario.CellUpdate) *agario.CellUpdate {
	var closest *agario.CellUpdate
	closestDist := math.MaxFloat64
	for _, cell := range ai.OwnCells {
		dist := dist2(p, cell.Point)
		if closest == nil {
			closest = cell
			closestDist = dist
			continue
		}

		if dist < closestDist || (dist == closestDist && cell.ID < closest.ID) {
			closest = cell
			closestDist = dist
		}
	}
	return closest
}

type filterFunc func(*agario.CellUpdate) bool

func (ai *AI) getClosestFiltered(p agario.Point, cells []*agario.CellUpdate, filter filterFunc) *agario.CellUpdate {
	var closest *agario.CellUpdate
	closestDist := math.MaxFloat64
	for _, cell := range ai.OwnCells {
		if !filter(cell) {
			continue
		}

		dist := dist2(p, cell.Point)
		if closest == nil {
			closest = cell
			closestDist = dist
			continue
		}

		if dist < closestDist || (dist == closestDist && cell.ID < closest.ID) {
			closest = cell
			closestDist = dist
		}
	}
	return closest
}

func (ai *AI) getSmallestCell() *agario.CellUpdate {
	var smallest *agario.CellUpdate
	for _, cell := range ai.OwnCells {
		if smallest == nil || cell.Size < smallest.Size || (cell.Size == smallest.Size && cell.ID < smallest.ID) {
			smallest = cell
		}
	}
	return smallest
}

func (ai *AI) getSpeed(cell *agario.CellUpdate) float64 {
	return 745.28 * math.Pow(float64(cell.Size), -0.222) * 50 / 1000
}

func (ai *AI) updateOwnCells() {
	ai.OwnCells = make([]*agario.CellUpdate, 0, len(ai.g.Game.MyIDs))
	for id := range ai.g.Game.MyIDs {
		cell, found := ai.g.Game.Cells[id]
		if !found {
			continue
		}
		ai.OwnCells = append(ai.OwnCells, cell)
	}
}

func dist2(a, b agario.Point) float64 {
	x1, y1 := float64(a.X), float64(a.Y)
	x2, y2 := float64(b.X), float64(b.Y)

	return square(x2-x1) + square(y2-y1)
}

func square(a float64) float64 {
	return a * a
}
