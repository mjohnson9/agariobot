package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/gonum/graph"
	"github.com/gonum/graph/search"
	"github.com/nightexcessive/agario"
)

const (
	eatSizeRequirement = 1.25
)

const (
	stateFleeing = iota
	stateHunting = iota
	stateFeeding = iota
	stateIdle    = iota
)

type AI struct {
	g *game

	Me              *agario.Cell
	SmallestOwnCell *agario.Cell

	State  byte
	Status []string
	Path   []mgl32.Vec2

	Map         Map
	DijkstraMap search.Shortest

	timeToNextSplit time.Duration

	OwnCells []*agario.Cell

	Predators []*agario.Cell
	Prey      []*agario.Cell
	Food      []*agario.Cell
}

const foodMaxSize = 20

func (ai *AI) Update(dt time.Duration) {
	if ai.timeToNextSplit > 0 {
		ai.timeToNextSplit -= dt
	}

	ai.Status = ai.Status[0:0]

	ai.updateOwnCells()

	ai.Me = ai.getPseudoMe()
	ai.SmallestOwnCell = ai.getSmallestOwnCell()

	ai.Predators = ai.Predators[0:0]
	ai.Prey = ai.Prey[0:0]
	ai.Food = ai.Food[0:0]

	predatorSize := int32(float32(ai.SmallestOwnCell.Size)*eatSizeRequirement) - 1

	preySize := int32(float32(ai.SmallestOwnCell.Size) / eatSizeRequirement)
	ignoreSize := ai.SmallestOwnCell.Size / 4

	for _, cell := range ai.g.Game.Cells {
		if cell.IsVirus {
			continue
		}

		if _, myCell := ai.g.Game.MyIDs[cell.ID]; myCell {
			continue
		}

		switch {
		case cell.Size <= 20: // Food. Players start at 10 and can't fall below it.
			ai.Food = append(ai.Food, cell)
		case cell.Size <= preySize && cell.Size >= ignoreSize:
			ai.Prey = append(ai.Prey, cell)
		case cell.Size >= predatorSize:
			ai.Predators = append(ai.Predators, cell)
		}
	}

	sort.Sort(sort.Reverse(cellArray(ai.Food)))
	sort.Sort(sort.Reverse(cellArray(ai.Prey)))
	sort.Sort(sort.Reverse(cellArray(ai.Predators)))

	ai.buildCostMap()
	ai.DijkstraMap = search.DijkstraFrom(ai.Map.GetNode(gameToCostMap(ai.Me.Position.Elem())), UndirectedMap(ai.Map), nil)

	ai.Execute()
}

func (ai *AI) Execute() {
	/*if len(ai.Predators) > 0 {
		isFleeing := ai.flee()
		if isFleeing {
			ai.State = stateFleeing
			return
		}
	} else {
		ai.addStatusMessage("Not fleeing: No known predators")
	}*/
	ai.addStatusMessage("Not fleeing: Disabled")

	if len(ai.Prey) > 0 {
		isHunting := ai.hunt()
		if isHunting {
			ai.State = stateHunting
			return
		}

		isChasing := ai.chase()
		if isChasing {
			ai.State = stateHunting
			return
		}
	} else {
		ai.addStatusMessage("Not hunting: No known prey")
		ai.addStatusMessage("Not chasing: No known prey")
	}

	if len(ai.Food) > 0 {
		isFeeding := ai.feed()
		if isFeeding {
			ai.State = stateFeeding
			return
		}
	} else {
		ai.addStatusMessage("Not feeding: No known food")
	}

	ai.addStatusMessage("Wandering")
	mapCenter := mgl32.Vec2{float32(ai.g.Game.Board.Bottom) / 2, float32(ai.g.Game.Board.Right) / 2}
	ai.movePathed(mapCenter)
	ai.State = stateIdle
}

// flee moves directly away from the nearest cell that is capable of eating us
func (ai *AI) flee() bool {
	canBeSplitKilledBySize := int32((float64(ai.SmallestOwnCell.Size)*eatSizeRequirement - 10) * 2)
	ignoreSplitKillSize := ai.SmallestOwnCell.Size * 4 // If they're 4x larger than us, they're unlikely to split to kill us
	closestDangerousPredator := ai.getClosestFiltered(ai.Me.Position, ai.Predators, func(cell *agario.Cell) bool {
		dist := dist2(ai.Me.Position, cell.Position)

		eatDistance := square((float32(cell.Size) - float32(ai.Me.Size)*0.35) + 100)
		if dist <= eatDistance {
			return true
		}
		if cell.Size < canBeSplitKilledBySize || cell.Size >= ignoreSplitKillSize {
			return false
		}

		splitDistance := cell.SplitDistance()
		return dist2(ai.Me.Position, cell.Position) <= splitDistance
	})
	if closestDangerousPredator == nil {
		ai.addStatusMessage("Not fleeing: No dangerous predators nearby")
		return false
	}

	// Find angle between us and the predator
	delta := closestDangerousPredator.Position.Sub(ai.Me.Position)
	angle := math.Atan2(float64(delta.X()), float64(delta.Y()))

	if angle > math.Pi {
		angle -= math.Pi
	} else {
		angle += math.Pi
	}

	// Position to move to
	targetX := ai.Me.Position.X() + float32(500*math.Sin(angle))
	targetY := ai.Me.Position.Y() + float32(500*math.Cos(angle))

	ai.addStatusMessage("Fleeing from " + prettyCellName(closestDangerousPredator))
	ai.movePathed(mgl32.Vec2{targetX, targetY})

	return true
}

// hunt attempts to kill the closest cell that can be killed by splitting
func (ai *AI) hunt() bool {
	if len(ai.OwnCells) > 1 {
		ai.addStatusMessage("Not hunting: Too many splits")
		// We won't intentionally split into more than two
		return false
	}
	if ai.SmallestOwnCell.Size <= 36 {
		ai.addStatusMessage("Not hunting: Too small")
		// We can't split unless we have at least 36 mass
		return false
	}

	canSplitKillSize := int32(float32(ai.SmallestOwnCell.Size) / 2 / eatSizeRequirement)
	splitDistance := square(4*(40+(ai.SmallestOwnCell.Speed()*4)) + (float32(ai.SmallestOwnCell.Size) * 1.75))

	closestPrey := ai.getClosestFiltered(ai.Me.Position, ai.Prey, func(cell *agario.Cell) bool {
		return cell.Size <= canSplitKillSize && dist2(ai.Me.Position, cell.Position) < splitDistance
	})
	if closestPrey == nil {
		ai.addStatusMessage("Not hunting: No prey to split kill")
		return false
	}

	ai.addStatusMessage("Splitting on " + prettyCellName(closestPrey))
	ai.Path = []mgl32.Vec2{ai.Me.Position, closestPrey.Position}
	ai.g.Game.SetTargetPos(closestPrey.Position.X(), closestPrey.Position.Y())
	if ai.timeToNextSplit <= 0 {
		ai.g.Game.Split()
		ai.timeToNextSplit = 250 * time.Millisecond
	}

	return true
}

// chase attempts to eat another blob by getting close enough to split on it
func (ai *AI) chase() bool {
	/*canKillSize := int16(float64(me.Size) / 2 / eatSizeRequirement)
	splitDistance := square(4*(40+(ai.getSpeed(me)*4)) + (float64(me.Size) * 1.75))*/

	closestPrey := ai.getClosest(ai.Me.Position, ai.Prey)
	if closestPrey == nil {
		ai.addStatusMessage("Not chasing: No prey")
		return false
	}

	ai.addStatusMessage("Chasing " + prettyCellName(closestPrey))
	ai.movePathed(closestPrey.Position)

	return true
}

// feed attempts to eat the nearest food cell
func (ai *AI) feed() bool {
	/*if ai.Me.Size >= 150 {
		ai.addStatusMessage("Not feeding: Too large")
		return false
	}*/

	closestFood := ai.getClosest(ai.Me.Position, ai.Food)
	if closestFood == nil {
		ai.addStatusMessage("Not feeding: No food pellets")
		return false
	}

	ai.addStatusMessage("Eating food pellets")
	ai.movePathed(closestFood.Position)

	return true
}

const (
	costMapReduction = 125

	costDoNotPass = 1024
)

func (ai *AI) buildCostMap() {
	w, h := int(ai.g.Game.Board.Right/costMapReduction), int(ai.g.Game.Board.Bottom/costMapReduction)
	ai.Map = NewMap(w+1, h+1)

	for x := 0; x < w; x++ {
		ai.Map[x][0] = costDoNotPass / 2
		ai.Map[x][1] += costDoNotPass / 3
		ai.Map[x][2] += costDoNotPass / 4
		ai.Map[x][3] += costDoNotPass / 5

		ai.Map[x][h] = costDoNotPass / 2
		ai.Map[x][h-1] = costDoNotPass / 3
		ai.Map[x][h-2] = costDoNotPass / 4
		ai.Map[x][h-3] = costDoNotPass / 5
	}

	for y := 0; y < h; y++ {
		ai.Map[0][y] = costDoNotPass / 2
		ai.Map[1][y] = costDoNotPass / 3
		ai.Map[2][y] = costDoNotPass / 4
		ai.Map[3][y] = costDoNotPass / 5

		ai.Map[w][y] = costDoNotPass / 2
		ai.Map[w-1][y] = costDoNotPass / 3
		ai.Map[w-2][y] = costDoNotPass / 4
		ai.Map[w-3][y] = costDoNotPass / 5
	}

	//canBeSplitKilledBySize := int32((float64(ai.SmallestOwnCell.Size)*eatSizeRequirement - 10) * 2)
	//ignoreSplitKillSize := ai.Me.Size * 4 // If they're 4x larger than us, they're unlikely to split to kill us

	for _, cell := range ai.Predators {
		x, y := gameToCostMap(cell.Position.Elem())

		/*if cell.Size >= canBeSplitKilledBySize && cell.Size <= ignoreSplitKillSize {
			splitDistance := int(cell.SplitDistance()+100) / costMapReduction
			setCostMapCircle(ai.Map, x, y, splitDistance, costDoNotPass/2)
			setCostMapCircle(ai.Map, x+1, y+1, splitDistance, costDoNotPass/2)
		}*/

		size := (int(cell.Size) + 100) / costMapReduction
		setCostMapCircle(ai.Map, x, y, size, costDoNotPass)
		setCostMapCircle(ai.Map, x+1, y+1, size, costDoNotPass)
	}
}

func setCostMapLine(m Map, x1, y1, x2, y2 int, value float32) {
	if x1 != x2 && y1 != y2 {
		panic("we can only draw straight lines")
	}

	w, h := m.width(), m.height()
	if x1 != x2 {
		if y1 < 0 || y1 >= h {
			return
		}

		if x1 > x2 {
			x1, x2 = x2, x1
		}
		if x1 < 0 {
			x1 = 0
		}
		if x2 >= w {
			x2 = w - 1
		}

		for x := x1; x <= x2; x++ {
			m[x][y1] = value
		}
	} else {
		if x1 < 0 || x1 >= w {
			return
		}
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		if y1 < 0 {
			y1 = 0
		}
		if y2 >= h {
			y2 = h - 1
		}
		for y := y1; y <= y2; y++ {
			m[x1][y] = value
		}
	}
}

func setCostMapCircle(m Map, pX, pY int, radius int, value float32) {
	x := radius
	y := 0
	decisionOver2 := 1 - x

	for x >= y {
		setCostMapLine(m, x+pX, y+pY, -x+pX, y+pY, value)
		setCostMapLine(m, x+pX, -y+pY, -x+pX, -y+pY, value)

		setCostMapLine(m, y+pX, x+pY, -y+pX, x+pY, value)
		setCostMapLine(m, y+pX, -x+pY, -y+pX, -x+pY, value)

		y++
		if decisionOver2 <= 0 {
			decisionOver2 += 2*y + 1
		} else {
			x--
			decisionOver2 += 2*(y-x) + 1
		}
	}
}

type filterFunc func(*agario.Cell) bool

func (ai *AI) cellsUntil(cells []*agario.Cell, filter filterFunc) func(graph.Node, int) bool {
	cellPositions := make(map[int]struct{})

	for _, c := range cells {
		if filter != nil && !filter(c) {
			continue
		}
		node := ai.Map.GetNode(gameToCostMap(c.Position.X(), c.Position.Y()))
		cellPositions[node.ID()] = struct{}{}
	}

	return func(n graph.Node, d int) bool {
		_, ok := cellPositions[n.ID()]

		return ok
	}
}

func (ai *AI) getClosest(p mgl32.Vec2, cells []*agario.Cell) *agario.Cell {
	return ai.getClosestFiltered(p, cells, nil)
}

func (ai *AI) getClosestFiltered(p mgl32.Vec2, cells []*agario.Cell, filter filterFunc) *agario.Cell {
	/*var closest *agario.Cell
	closestDist := float32(math.MaxFloat32)
	for _, cell := range cells {
		if filter != nil && !filter(cell) {
			continue
		}

		dist := dist2(p, cell.Position)
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
	return closest*/

	if len(cells) == 0 {
		return nil
	}

	var (
		shortest     *agario.Cell
		shortestCost = math.MaxFloat64
	)
	for _, c := range cells {
		if filter != nil && !filter(c) {
			continue
		}

		n := ai.Map.GetNode(gameToCostMap(c.Position.Elem()))
		cost := ai.DijkstraMap.WeightTo(n)
		/*if cost >= costDoNotPass {
			continue
		}*/
		if shortest == nil || cost < shortestCost || (mgl64.FloatEqual(cost, shortestCost) && c.ID < shortest.ID) {
			shortest = c
			shortestCost = cost
		}
	}

	return shortest

	/*bfs := new(traverse.BreadthFirst)
	visited := 0
	bfs.Visit = func(u, v graph.Node) {
		visited++
	}

	n := bfs.Walk(ai.Map, ai.Map.GetNode(gameToCostMap(p.X(), p.Y())), ai.cellsUntil(cells, filter))
	ai.addStatusMessage("BFS: visited " + strconv.Itoa(visited) + " nodes")
	if n == nil {
		return nil
	}

	mN := n.(*mapNode)

	minX, minY := costMapToGame(mN.X, mN.Y)

	//maxX := (minX + 1) * costMapReduction
	//maxY := (minY + 1) * costMapReduction
	maxX, maxY := costMapToGame(mN.X+1, mN.Y+1)

	for _, c := range cells {
		x := c.Position.X()
		y := c.Position.Y()

		if x >= minX && x < maxX && y >= minY && y < maxY {
			return c
		}
	}

	return nil*/
}

func (ai *AI) getPseudoMe() *agario.Cell {
	firstCell := ai.OwnCells[0]
	me := &agario.Cell{
		ID:   firstCell.ID,
		Name: firstCell.Name,

		Heading: firstCell.Heading,

		Color: firstCell.Color,

		IsVirus: firstCell.IsVirus,
	}
	var avgPosition mgl32.Vec2
	for _, cell := range ai.OwnCells {
		me.Size += cell.Size

		if avgPosition.X() == 0 && avgPosition.Y() == 0 {
			avgPosition = cell.Position
			continue
		}

		avgPosition = avgPosition.Add(cell.Position)
	}

	n := float32(len(ai.OwnCells))
	avgPosition[0] = avgPosition[0] / n
	avgPosition[1] = avgPosition[1] / n
	me.Position = avgPosition

	return me
}

func (ai *AI) getSmallestOwnCell() *agario.Cell {
	var smallest *agario.Cell
	for _, cell := range ai.OwnCells {
		if smallest == nil {
			smallest = cell
			continue
		}

		if smallest.Size < cell.Size || (smallest.Size == cell.Size && cell.ID < smallest.ID) {
			smallest = cell
		}
	}
	return smallest
}

func (ai *AI) getOurTotalSize() (s int32) {
	for _, cell := range ai.OwnCells {
		s += cell.Size
	}
	return
}

func (ai *AI) updateOwnCells() {
	ai.OwnCells = make([]*agario.Cell, 0, len(ai.g.Game.MyIDs))
	for id := range ai.g.Game.MyIDs {
		cell, found := ai.g.Game.Cells[id]
		if !found {
			continue
		}
		ai.OwnCells = append(ai.OwnCells, cell)
	}
}

func (ai *AI) addStatusMessage(str string) {
	ai.Status = append(ai.Status, str)
}

/*func (ai *AI) movePathedUndirected(position mgl32.Vec2) {
	undirectedMap := UndirectedMap(ai.Map)

	meNode := ai.Map.GetNode(gameToCostMap(ai.Me.Position.Elem()))
	positionNode := ai.Map.GetNode(gameToCostMap(position.Elem()))

	path, cost, nodes := search.AStar(meNode, positionNode, undirectedMap, nil, nil)
	if path == nil {
		ai.addStatusMessage("Failed to find path. Moving directly to objective.")

		ai.Path = []mgl32.Vec2{ai.Me.Position, position}
		ai.g.Game.SetTargetPos(position.X(), position.Y())
		return
	}
	ai.addStatusMessage(fmt.Sprintf("A* (undirected): path cost: %.2f / nodes expanded: %d", cost, nodes))

	ai.moveAlongPath(position, path)
}*/

func (ai *AI) movePathed(position mgl32.Vec2) {
	minDistance2 := square(float32(ai.Me.Size) + costMapReduction*1.3)

	if dist2(ai.Me.Position, position) < minDistance2 {
		ai.addStatusMessage("Objective is within minimum distance. Moving directly to objective.")

		ai.Path = []mgl32.Vec2{ai.Me.Position, position}
		ai.g.Game.SetTargetPos(position.X(), position.Y())
		return
	}

	// meNode := ai.Map.GetNode(gameToCostMap(ai.Me.Position.Elem()))
	// positionNode := ai.Map.GetNode(gameToCostMap(position.Elem()))

	// path, cost, nodes := search.AStar(meNode, positionNode, ai.Map, nil, nil)
	// if path == nil {
	// 	ai.addStatusMessage("Failed to find path. Trying undirected.")

	// 	ai.movePathedUndirected(position)
	// 	return
	// }
	// ai.addStatusMessage(fmt.Sprintf("A*: path cost: %.2f / nodes expanded: %d", cost, nodes))

	positionNode := ai.Map.GetNode(gameToCostMap(position.Elem()))
	path, cost := ai.DijkstraMap.To(positionNode)
	if path == nil {
		ai.addStatusMessage("movePathed: Failed to find path. Moving directly to objective.")

		ai.Path = []mgl32.Vec2{ai.Me.Position, position}
		ai.g.Game.SetTargetPos(position.X(), position.Y())
		return
	}

	ai.addStatusMessage(fmt.Sprintf("movePathed: path cost: %.2f", cost))

	ai.moveAlongPath(position, path)
}

func (ai *AI) moveAlongPath(targetPosition mgl32.Vec2, path []graph.Node) {
	minDistance2 := square(float32(ai.Me.Size) + costMapReduction*1.3)

	var pathNode *mapNode
	var pathVecs []mgl32.Vec2
	for _, rawNode := range path {
		node := rawNode.(*mapNode)
		pos := mgl32.Vec2{float32(node.X) * costMapReduction, float32(node.Y) * costMapReduction}

		if pathNode == nil && dist2(ai.Me.Position, pos) >= minDistance2 {
			pathNode = node
		}

		pathVecs = append(pathVecs, pos)
	}

	if pathNode == nil {
		ai.addStatusMessage("Failed to find path node that was far enough away. Moving directly to objective.")

		ai.Path = []mgl32.Vec2{ai.Me.Position, targetPosition}
		ai.g.Game.SetTargetPos(targetPosition.X(), targetPosition.Y())
		return
	}

	ai.Path = pathVecs
	ai.g.Game.SetTargetPos(float32(pathNode.X*costMapReduction), float32(pathNode.Y*costMapReduction))
}

func dist2(a, b mgl32.Vec2) float32 {
	diff := b.Sub(a)

	return square(diff.X()) + square(diff.Y())
}

func square(a float32) float32 {
	return a * a
}

func prettyCellName(cell *agario.Cell) string {
	if cell.Name != "" {
		return cell.Name + " (" + strconv.Itoa(int(cell.Size)) + ")"
	}

	return "an unnamed cell (" + strconv.Itoa(int(cell.Size)) + ")"
}

func costMapToGame(x, y int) (float32, float32) {
	return float32(x * costMapReduction), float32(y * costMapReduction)
}

func gameToCostMap(x, y float32) (int, int) {
	return int(x / costMapReduction), int(y / costMapReduction)
}
