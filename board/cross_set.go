package board

import (
	"github.com/domino14/macondo/alphabet"
	"github.com/domino14/macondo/gaddag"
	"github.com/domino14/macondo/gaddagmaker"
	"github.com/domino14/macondo/move"
)

const (
	// TrivialCrossSet allows every possible letter. It is the default
	// state of a square.
	TrivialCrossSet = (1 << alphabet.MaxAlphabetSize) - 1
)

// A CrossSet is a bit mask of letters that are allowed on a square. It is
// inherently directional, as it depends on which direction we are generating
// moves in. If we are generating moves HORIZONTALLY, we check in the
// VERTICAL cross set to make sure we can play a letter there.
// Therefore, a VERTICAL cross set is created by looking at the tile(s)
// above and/or below the relevant square and seeing what letters lead to
// valid words.
type CrossSet uint64

func (c CrossSet) Allowed(letter alphabet.MachineLetter) bool {
	return c&(1<<uint8(letter)) != 0
}

func (c *CrossSet) set(letter alphabet.MachineLetter) {
	*c = *c | (1 << letter)
}

func CrossSetFromString(letters string, alph *alphabet.Alphabet) CrossSet {
	c := CrossSet(0)
	for _, l := range letters {
		v, err := alph.Val(l)
		if err != nil {
			panic("Letter error: " + string(l))
		}
		c.set(v)
	}
	return c
}

func (c *CrossSet) setAll() {
	*c = TrivialCrossSet
}

func (c *CrossSet) clear() {
	*c = 0
}

func (b *GameBoard) updateCrossSetsForMove(m *move.Move, gd *gaddag.SimpleGaddag,
	bag *alphabet.Bag) {

	row, col, vertical := m.CoordsAndVertical()
	// Every tile placed by this new move creates new "across" words, and we need
	// to update the cross sets on both sides of these across words, as well
	// as the cross sets for THIS word.

	// Assumes all across words are HORIZONTAL.
	calcForAcross := func(rowStart int, colStart int, csd BoardDirection) {
		for row := rowStart; row < len(m.Tiles())+rowStart; row++ {
			if m.Tiles()[row-rowStart] == alphabet.PlayedThroughMarker {
				// No new "across word" was generated by this tile, so no need
				// to update cross set.
				continue
			}
			// Otherwise, look along this row. Note, the edge is still part
			// of the word.
			rightCol := b.wordEdge(int(row), int(colStart), RightDirection)
			leftCol := b.wordEdge(int(row), int(colStart), LeftDirection)
			b.GenCrossSet(int(row), int(rightCol)+1, csd, gd, bag)
			b.GenCrossSet(int(row), int(leftCol)-1, csd, gd, bag)
			// This should clear the cross set on the just played tile.
			b.GenCrossSet(int(row), int(colStart), csd, gd, bag)
		}
	}

	// assumes self is HORIZONTAL
	calcForSelf := func(rowStart int, colStart int, csd BoardDirection) {
		// Generate cross-sets on either side of the word.
		for col := int(colStart) - 1; col <= int(colStart)+len(m.Tiles()); col++ {
			b.GenCrossSet(int(rowStart), col, csd, gd, bag)
		}
	}

	if vertical {
		calcForAcross(row, col, HorizontalDirection)
		b.Transpose()
		row, col = col, row
		calcForSelf(row, col, VerticalDirection)
		b.Transpose()
	} else {
		calcForSelf(row, col, HorizontalDirection)
		b.Transpose()
		row, col = col, row
		calcForAcross(row, col, VerticalDirection)
		b.Transpose()
	}

}

// GenAllCrossSets generates all cross-sets. It goes through the entire
// board; our anchor algorithm doesn't quite match the one in the Gordon
// paper.
// We do this for both transpositions of the board.
func (b *GameBoard) GenAllCrossSets(gaddag *gaddag.SimpleGaddag, bag *alphabet.Bag) {

	n := b.Dim()
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			b.GenCrossSet(i, j, HorizontalDirection, gaddag, bag)
		}
	}
	b.Transpose()
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			b.GenCrossSet(i, j, VerticalDirection, gaddag, bag)
		}
	}
	// And transpose back to the original orientation.
	b.Transpose()
}

// GenCrossSet generates a cross-set for each individual square.
func (b *GameBoard) GenCrossSet(row int, col int, dir BoardDirection,
	gaddag *gaddag.SimpleGaddag, bag *alphabet.Bag) {

	if row < 0 || row >= b.Dim() || col < 0 || col >= b.Dim() {
		return
	}
	// If the square has a letter in it, its cross set and cross score
	// should both be 0
	if !b.squares[row][col].IsEmpty() {
		b.squares[row][col].setCrossSet(CrossSet(0), dir)
		b.squares[row][col].setCrossScore(0, dir)
		return
	}
	// If there's no tile adjacent to this square in any direction,
	// every letter is allowed.
	if b.leftAndRightEmpty(row, col) {
		b.squares[row][col].setCrossSet(TrivialCrossSet, dir)
		b.squares[row][col].setCrossScore(0, dir)

		return
	}
	// If we are here, there is a letter to the left, to the right, or both.
	// start from the right and go backwards.
	rightCol := b.wordEdge(row, col+1, RightDirection)
	if rightCol == col {
		// This means the right was always empty; we only want to go left.
		lNodeIdx, lPathValid := b.traverseBackwards(row, col-1,
			gaddag.GetRootNodeIndex(), false, 0, gaddag)
		score := b.traverseBackwardsForScore(row, col-1, bag)
		b.squares[row][col].setCrossScore(score, dir)

		if !lPathValid {
			// There are no further extensions to the word on the board,
			// which may also be a phony.
			b.squares[row][col].setCrossSet(CrossSet(0), dir)
			return
		}
		// Otherwise, we have a left node index.
		sIdx := gaddag.NextNodeIdx(lNodeIdx, alphabet.SeparationMachineLetter)
		// Take the letter set of this sIdx as the cross-set.
		letterSet := gaddag.GetLetterSet(sIdx)
		// Miraculously, letter sets and cross sets are compatible.
		b.squares[row][col].setCrossSet(CrossSet(letterSet), dir)
	} else {

		// Otherwise, the right is not empty. Check if the left is empty,
		// if so we just traverse right, otherwise, we try every letter.
		leftCol := b.wordEdge(row, col-1, LeftDirection)
		// Start at the right col and work back to this square.
		lNodeIdx, lPathValid := b.traverseBackwards(row, rightCol,
			gaddag.GetRootNodeIndex(), false, 0, gaddag)
		scoreR := b.traverseBackwardsForScore(row, rightCol, bag)
		scoreL := b.traverseBackwardsForScore(row, col-1, bag)
		b.squares[row][col].setCrossScore(scoreR+scoreL, dir)
		if !lPathValid {
			b.squares[row][col].setCrossSet(CrossSet(0), dir)
			return
		}
		if leftCol == col {
			// The left is empty, but the right isn't.
			// The cross-set is just the letter set of the letter directly
			// to our right.

			letterSet := gaddag.GetLetterSet(lNodeIdx)
			b.squares[row][col].setCrossSet(CrossSet(letterSet), dir)
		} else {
			// Both the left and the right have a tile. Go through the
			// siblings, from the right, to see what nodes lead to the left.

			numArcs := gaddag.NumArcs(lNodeIdx)
			crossSet := b.squares[row][col].getCrossSet(dir)
			*crossSet = CrossSet(0)
			for i := lNodeIdx + 1; i <= uint32(numArcs)+lNodeIdx; i++ {
				ml := alphabet.MachineLetter(gaddag.Nodes[i] >>
					gaddagmaker.LetterBitLoc)
				if ml == alphabet.SeparationMachineLetter {
					continue
				}
				nnIdx := gaddag.Nodes[i] & gaddagmaker.NodeIdxBitMask
				_, success := b.traverseBackwards(row, col-1, nnIdx, true,
					leftCol, gaddag)
				if success {
					crossSet.set(ml)
				}
			}
		}
	}
}