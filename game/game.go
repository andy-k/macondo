// Package game encapsulates the main mechanics for a Crossword Game. It
// interacts heavily with the protobuf data structures.
package game

import (
	"fmt"
	"math/rand"

	"github.com/domino14/macondo/alphabet"
	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/gaddag"
	"github.com/domino14/macondo/move"
	pb "github.com/domino14/macondo/rpc/api/proto"
	"github.com/lithammer/shortuuid"
	"github.com/rs/zerolog/log"
)

// RuleDefiner is an interface that is used for passing a set of rules
// to a game.
type RuleDefiner interface {
	Gaddag() *gaddag.SimpleGaddag
	Board() *board.GameBoard
	LetterDistribution() *alphabet.LetterDistribution

	LoadRule(lexiconName, letterDistributionName string) error
}

// // History is a wrapper around pb.GameHistory to allow for additional
// // methods and an actual array of positions.
// type History struct {
// 	pb.GameHistory

// }

// Position is a wrapper around pb.GamePosition that has additional
// methods and values associated with it, such as the current board state.
// type Position struct {
// 	pb.GamePosition

// 	// The pb.GamePosition doesn't know how to deal with alphabet.Rack,
// 	// so we add an extra field here just for the alphabet rack,
// 	// mapped 1-to-1 to the pb.GamePosition.Players array.
// 	playerRacks []*alphabet.Rack
// }

type playerState struct {
	pb.PlayerInfo

	rack        *alphabet.Rack
	rackLetters string
	points      int
}

func (p *playerState) resetScore() {
	p.points = 0
}

func (p *playerState) throwRackIn(bag *alphabet.Bag) {
	bag.PutBack(p.rack.TilesOn())
	p.rack.Set([]alphabet.MachineLetter{})
	p.rackLetters = ""
}

func (p *playerState) setRackTiles(tiles []alphabet.MachineLetter, alph *alphabet.Alphabet) {
	p.rack.Set(tiles)
	p.rackLetters = alphabet.MachineWord(tiles).UserVisible(alph)
}

type playerStates []*playerState

func (p playerStates) resetScore() {
	for idx := range p {
		p[idx].resetScore()
	}
}

func (p playerStates) flipFirst() {
	p[0], p[1] = p[1], p[0]
	p[0].Number = 1
	p[1].Number = 2
}

// Game is the actual internal game structure that controls the entire
// business logic of the game; drawing, making moves, etc. The two
// structures above are basically data entities.
// Note: a Game doesn't care how it is played. It is just rules for gameplay.
// AI players, human players, etc will play a game outside of the scope of
// this module.
type Game struct {
	gaddag *gaddag.SimpleGaddag
	alph   *alphabet.Alphabet
	// board and bag will contain the latest (current) versions of these.
	board              *board.GameBoard
	letterDistribution *alphabet.LetterDistribution
	bag                *alphabet.Bag

	playing bool

	randSeed   int64
	randSource *rand.Rand

	scorelessTurns int
	onturn         int
	turnnum        int
	players        playerStates
	// history has a history of all the moves in this game. Note that
	// history only gets written to when someone plays a move that is NOT
	// backed up.
	history *pb.GameHistory

	stateStack []*stateBackup
	stackPtr   int
}

// CalculateCoordsFromStringPosition turns a "position" on the board such as
// H7 and turns it into a numeric row, col, and direction.
func CalculateCoordsFromStringPosition(evt *pb.GameEvent) {
	// Note that this evt.Position has nothing to do with the type Position
	// we are defining in this package.
	row, col, vertical := move.FromBoardGameCoords(evt.Position)
	if vertical {
		evt.Direction = pb.GameEvent_VERTICAL
	} else {
		evt.Direction = pb.GameEvent_HORIZONTAL
	}
	evt.Row = int32(row)
	evt.Column = int32(col)
}

func newHistory(players playerStates) *pb.GameHistory {
	his := &pb.GameHistory{}

	playerInfo := make([]*pb.PlayerInfo, len(players))
	for idx, p := range players {
		playerInfo[idx] = &pb.PlayerInfo{Nickname: p.Nickname,
			RealName: p.RealName,
			Number:   p.Number}
	}

	his.Players = playerInfo
	his.Uuid = shortuuid.New()
	his.Turns = []*pb.GameTurn{}
	return his
}

// NewGame is how one instantiates a brand new game.
func NewGame(rules RuleDefiner, playerinfo []*pb.PlayerInfo) (*Game, error) {
	game := &Game{}
	game.gaddag = rules.Gaddag()
	game.alph = game.gaddag.GetAlphabet()
	game.letterDistribution = rules.LetterDistribution()

	game.board = rules.Board().Copy()

	game.players = make([]*playerState, len(playerinfo))
	for idx, p := range playerinfo {
		game.players[idx] = &playerState{
			PlayerInfo: pb.PlayerInfo{
				Nickname: p.Nickname,
				Number:   p.Number,
				RealName: p.RealName},
		}
	}

	return game, nil
}

// NewFromHistory instantiates a Game from a history, and sets the current
// turn to the passed in turnnum. It assumes the rules contains the current
// lexicon in history, if any!
func NewFromHistory(history *pb.GameHistory, rules RuleDefiner, turnnum int) (*Game, error) {
	game, err := NewGame(rules, history.Players)
	if err != nil {
		return nil, err
	}
	game.history = history

	// Initialize the bag and player rack structures to avoid panics.
	game.randSeed, game.randSource = seededRandSource()
	log.Debug().Msgf("Random seed for this game was %v", game.randSeed)
	game.bag = game.letterDistribution.MakeBag(game.randSource)
	for i := 0; i < game.NumPlayers(); i++ {
		game.players[i].rack = alphabet.NewRack(game.alph)
	}
	// Then play to the passed-in turn.
	err = game.PlayToTurn(turnnum)
	if err != nil {
		return nil, err
	}
	return game, nil
}

// StartGame seeds the random source anew, and starts a game, dealing out tiles
// to both players.
func (g *Game) StartGame() {
	g.Board().Clear()
	g.randSeed, g.randSource = seededRandSource()
	log.Debug().Msgf("Random seed for this game was %v", g.randSeed)
	g.bag = g.letterDistribution.MakeBag(g.randSource)

	flipFirst := g.randSource.Intn(2)
	if flipFirst == 0 {
		g.players.flipFirst()
	}
	g.history = newHistory(g.players)
	// Deal out tiles
	for i := 0; i < g.NumPlayers(); i++ {
		tiles, err := g.bag.Draw(7)
		if err != nil {
			panic(err)
		}
		g.players[i].rack = alphabet.NewRack(g.alph)
		g.players[i].setRackTiles(tiles, g.alph)
		g.players[i].points = 0
	}
	g.playing = true
}

// PlayMove plays a move on the board. This function is meant to be used
// by simulators as it implements a subset of possible moves.
// XXX: It doesn't implement special things like challenge bonuses, etc.
// XXX: Will this still be true, or should this function do it all?
func (g *Game) PlayMove(m *move.Move, backup bool) {
	var turn *pb.GameTurn

	// if we are backing up, then we do not want to add a new turn to the
	// game history. We only back up when we are simulating / generating endgames / etc,
	// and we only want to add new turns during actual gameplay.
	if backup {
		g.backupState()
	} else {
		turn = &pb.GameTurn{Events: []*pb.GameEvent{}}
	}
	switch m.Action() {
	case move.MoveTypePlay:
		g.board.PlayMove(m, g.gaddag, g.bag.LetterDistribution())
		score := m.Score()
		if score != 0 {
			g.scorelessTurns = 0
		}
		g.players[g.onturn].points += score

		drew := g.bag.DrawAtMost(m.TilesPlayed())
		tiles := append(drew, []alphabet.MachineLetter(m.Leave())...)
		g.players[g.onturn].setRackTiles(tiles, g.alph)

		if !backup {
			turn.Events = append(turn.Events, g.eventFromMove(m))
		}

		if g.players[g.onturn].rack.NumTiles() == 0 {
			g.playing = false
			unplayedPts := g.calculateRackPts((g.onturn+1)%len(g.players)) * 2
			g.players[g.onturn].points += unplayedPts
			if !backup {
				turn.Events = append(turn.Events, g.endRackEvt(unplayedPts))
			}
		}

	case move.MoveTypePass:
		g.scorelessTurns++
		if !backup {
			turn.Events = append(turn.Events, g.eventFromMove(m))
		}

	case move.MoveTypeExchange:
		drew, err := g.bag.Exchange([]alphabet.MachineLetter(m.Tiles()))
		if err != nil {
			panic(err)
		}
		tiles := append(drew, []alphabet.MachineLetter(m.Leave())...)
		g.players[g.onturn].setRackTiles(tiles, g.alph)
		g.scorelessTurns++
		if !backup {
			turn.Events = append(turn.Events, g.eventFromMove(m))
		}
	}

	if g.scorelessTurns == 6 {
		g.playing = false
		// Take away pts on each player's rack.
		for i := 0; i < len(g.players); i++ {
			pts := g.calculateRackPts(i)
			g.players[i].points -= pts
		}
		// XXX: add rack penalty for each player to the history.
	}

	if !backup {
		g.addToHistory(turn)
	}

	g.onturn = (g.onturn + 1) % len(g.players)
	g.turnnum++
}

// PlayScoringMove plays a move on a board that is described by the
// coordinates and word only. It returns the move.
func (g *Game) PlayScoringMove(coords, word string) (*move.Move, error) {
	playerid := g.onturn
	rack := g.RackFor(playerid).String()

	m, err := g.CreateAndScorePlacementMove(coords, word, rack)
	if err != nil {
		log.Error().Msgf("Trying to create and score move, err was %v", err)
		return nil, err
	}
	// Actually make the play on the board:
	g.PlayMove(m, false)
	return m, nil
}

// CreateAndScorePlacementMove creates a *move.Move from the coords and
// given tiles. It scores the move, calculates the leave, etc. This should
// be used when a person is interacting with the interface.
func (g *Game) CreateAndScorePlacementMove(coords string, tiles string, rack string) (*move.Move, error) {

	row, col, vertical := move.FromBoardGameCoords(coords)

	// convert tiles to MachineWord
	mw, err := alphabet.ToMachineWord(tiles, g.alph)
	if err != nil {
		return nil, err
	}
	rackmw, err := alphabet.ToMachineWord(rack, g.alph)
	if err != nil {
		return nil, err
	}
	tilesPlayed := 0
	for _, m := range mw {
		if m.IsPlayedTile() {
			tilesPlayed++
		}
	}
	leavemw, err := Leave(rackmw, mw)
	if err != nil {
		return nil, err
	}
	err = g.Board().ErrorIfIllegalPlay(row, col, vertical, mw)
	if err != nil {
		return nil, err
	}
	// Notes: the cross direction is in the opposite direction that the
	// play is actually in. Additionally, we transpose the board if
	// the play is vertical, due to how the scoring routine works.
	// We transpose it back at the end.
	crossDir := board.VerticalDirection
	if vertical {
		crossDir = board.HorizontalDirection
		row, col = col, row
		g.Board().Transpose()
	}

	// ScoreWord assumes the play is always horizontal, so we have to
	// do the transpositions beforehand.
	score := g.Board().ScoreWord(mw, row, col, tilesPlayed, crossDir, g.bag.LetterDistribution())
	// reset row, col back for the actual creation of the play.
	if vertical {
		row, col = col, row
		g.Board().Transpose()
	}
	m := move.NewScoringMove(score, mw, leavemw, vertical, tilesPlayed,
		g.alph, row, col, coords)
	return m, nil

}

func (g *Game) calculateRackPts(onturn int) int {
	rack := g.players[onturn].rack
	return rack.ScoreOn(g.bag.LetterDistribution())
}

func (g *Game) addToHistory(turn *pb.GameTurn) {
	g.history.Turns = append(g.history.Turns, turn)
}

func otherPlayer(idx int) int {
	return (idx + 1) % 2
}

func (g *Game) PlayToTurn(turnnum int) error {
	if turnnum < 0 || turnnum > len(g.history.Turns) {
		return fmt.Errorf("game has %v turns, you have chosen a turn outside the range",
			len(g.history.Turns))
	}
	if g.board == nil {
		return fmt.Errorf("board does not exist")
	}
	if g.bag == nil {
		return fmt.Errorf("bag has not been initialized; need to start game")
	}

	g.board.Clear()
	g.bag.Refill()
	g.players.resetScore()
	g.turnnum = 0
	g.onturn = 0
	g.playing = true
	playedTiles := []alphabet.MachineLetter(nil)
	var t int
	for t = 0; t < turnnum; t++ {
		addlTiles := g.playTurn(t)
		playedTiles = append(playedTiles, addlTiles...)
	}
	// err := g.reconcileTiles(playedTiles, t)
	// if err != nil {
	// 	return err
	// }
	if g.history.LastKnownRacks != nil {
		g.SetRacksForBoth([]*alphabet.Rack{
			alphabet.RackFromString(g.history.LastKnownRacks[0], g.alph),
			alphabet.RackFromString(g.history.LastKnownRacks[1], g.alph),
		})
	} else {
		// playTurn should have refilled the rack of the relevant player,
		// who was on turn. So draw a random rack for the player who's on
		// turn now.
		g.SetRandomRack(g.onturn)
	}
	for _, p := range g.players {
		if p.rack.NumTiles() == 0 {
			log.Debug().Msgf("Player %v has no tiles, game is over.", p)
			g.playing = false
			break
		}
	}
	return nil
	// clear the board, refill the bag
	// reset score.. see PlayGameToTurn in game_repr.go
	// update the bag
	// when we reach the final turn, assign a random rack from undrawn letters
	// to opponent automatically (if not specified in the history as incomplete racks)
	// (see LastKnownRacks field)
}

func (g *Game) playTurn(t int) []alphabet.MachineLetter {
	// XXX: This function is pretty similar to PlayMove above. It has a
	// subset of the functionality as it's designed to replay an already
	// recorded turn on the board.
	playedTiles := []alphabet.MachineLetter(nil)
	challengedOffPlay := false
	evts := g.history.Turns[t].Events
	// Check for the special case where a player played a phony that was
	// challenged off. We don't want to process this at all.
	if len(evts) == 2 {
		if evts[0].Type == pb.GameEvent_TILE_PLACEMENT_MOVE &&
			evts[1].Type == pb.GameEvent_PHONY_TILES_RETURNED {
			challengedOffPlay = true
		}
	}
	// Set the rack for the user on turn to the rack in the history.
	g.SetRackFor(g.onturn, alphabet.RackFromString(evts[0].Rack, g.alph))
	if !challengedOffPlay {
		for _, evt := range evts {
			m := moveFromEvent(evt, g.alph)

			switch m.Action() {
			case move.MoveTypePlay:
				g.board.PlayMove(m, g.gaddag, g.bag.LetterDistribution())
				g.players[g.onturn].points += m.Score()

				// Add tiles to playedTilesList
				for _, t := range m.Tiles() {
					if t != alphabet.PlayedThroughMarker {
						// Note that if a blank is played, the blanked letter
						// is added to the played tiles (and not the blank itself)
						// The RemoveTiles function below handles this later.
						playedTiles = append(playedTiles, t)
					}
				}
				// Note that what we draw here (and in exchange, below) may not
				// be what was recorded. That's ok -- we always set the rack
				// at the beginning to whatever was recorded. Drawing like
				// normal, though, ensures we don't have to reconcile any
				// tiles with the bag.
				drew := g.bag.DrawAtMost(m.TilesPlayed())
				tiles := append(drew, []alphabet.MachineLetter(m.Leave())...)
				g.players[g.onturn].setRackTiles(tiles, g.alph)

				// Don't check game end logic here, as we assume we have the
				// right event for that (move.MoveTypeEndgameTiles for example).

			case move.MoveTypeChallengeBonus, move.MoveTypeEndgameTiles,
				move.MoveTypePhonyTilesReturned, move.MoveTypeLostTileScore,
				move.MoveTypeLostScoreOnTime:

				// The score should already have the proper sign at creation time.
				g.players[g.onturn].points += m.Score()

			case move.MoveTypeExchange:

				drew, err := g.bag.Exchange([]alphabet.MachineLetter(m.Tiles()))
				if err != nil {
					panic(err)
				}
				tiles := append(drew, []alphabet.MachineLetter(m.Leave())...)
				g.players[g.onturn].setRackTiles(tiles, g.alph)
			default:
				// Nothing

			}
		}
	}

	g.onturn = (g.onturn + 1) % len(g.players)
	g.turnnum++
	return playedTiles
}

// func (g *Game) reconcileTiles(playedTiles []alphabet.MachineLetter, turn int) error {
// 	// Reconcile tiles in the bag.

// 	var err error
// 	log.Debug().Msgf("Removing tiles played: %v", playedTiles)
// 	err = g.bag.RemoveTiles(playedTiles)
// 	log.Debug().Msgf("Error was %v", err)
// 	if err != nil {
// 		return err
// 	}
// 	// Determine the latest rack available. Sometimes a game representation
// 	// ends before the game itself is over, so we need to make sure
// 	// we don't mistakenly give the other player all the remaining tiles, etc
// 	var rack alphabet.MachineWord
// 	notOnTurn := (g.onturn + 1) % 2

// 	if turn < len(repr.Turns) {
// 		// Find the rack of the player currently on turn; after playing
// 		// turns up to this point.
// 		rack, err = alphabet.ToMachineWord(repr.Turns[turn][0].GetRack(), g.alph)

// 		if err != nil {
// 			return err
// 		}
// 	}
// 	// Always empty the opponent's rack, so it doesn't display.
// 	g.players[notOnTurn].SetRack([]alphabet.MachineLetter{}, g.alph)
// 	log.Debug().Msgf("My rack is %v, removing it from bag", rack.UserVisible(g.alph))
// 	g.players[g.onturn].SetRack(rack, g.alph)

// 	return g.bag.RemoveTiles(rack)
// }

// SetRackFor sets the player's current rack. It throws an error if
// the rack is impossible to set from the current unseen tiles. It
// puts tiles back from opponent racks and our own racks, then sets the rack,
// and finally redraws for opponent.
func (g *Game) SetRackFor(playerIdx int, rack *alphabet.Rack) error {
	// Put our tiles back in the bag, as well as our opponent's tiles.
	g.ThrowRacksIn()

	// Check if we can actually set our rack now that these tiles are in the
	// bag.
	err := g.bag.RemoveTiles(rack.TilesOn())
	if err != nil {
		log.Error().Msgf("Unable to set rack: %v", err)
		return err
	}

	// success; set our rack
	g.players[playerIdx].rack = rack
	g.players[playerIdx].rackLetters = rack.String()
	// And redraw a random rack for opponent.
	g.SetRandomRack(otherPlayer(playerIdx))

	return nil
}

// SetRacksForBoth sets both racks at the same time.
func (g *Game) SetRacksForBoth(racks []*alphabet.Rack) error {
	g.ThrowRacksIn()
	for _, rack := range racks {
		err := g.bag.RemoveTiles(rack.TilesOn())
		if err != nil {
			log.Error().Msgf("Unable to set rack: %v", err)
			return err
		}
	}
	for idx, player := range g.players {
		player.rack = racks[idx]
		player.rackLetters = racks[idx].String()
	}
	return nil
}

// ThrowRacksIn throws both players' racks back in the bag.
func (g *Game) ThrowRacksIn() {
	g.players[0].throwRackIn(g.bag)
	g.players[1].throwRackIn(g.bag)
}

// SetRandomRack sets the player's rack to a random rack drawn from the bag.
// It tosses the current rack back in first. This is used for simulations.
func (g *Game) SetRandomRack(playerIdx int) {
	tiles := g.bag.Redraw(g.RackFor(playerIdx).TilesOn())
	g.players[playerIdx].setRackTiles(tiles, g.alph)
}

// RackFor returns the rack for the player with the passed-in index
func (g *Game) RackFor(playerIdx int) *alphabet.Rack {
	return g.players[playerIdx].rack
}

// RackLettersFor returns a user-visible representation of the player's rack letters
func (g *Game) RackLettersFor(playerIdx int) string {
	return g.RackFor(playerIdx).String()
}

// PointsFor returns the number of points for the given player
func (g *Game) PointsFor(playerIdx int) int {
	return g.players[playerIdx].points
}

// NumPlayers is always 2.
func (g *Game) NumPlayers() int {
	return 2
}

// Bag returns the current bag
func (g *Game) Bag() *alphabet.Bag {
	return g.bag
}

// Board returns the current board state.
func (g *Game) Board() *board.GameBoard {
	return g.board
}

func (g *Game) Turn() int {
	return g.turnnum
}

func (g *Game) Uuid() string {
	return g.history.Uuid
}

func (g *Game) Playing() bool {
	return g.playing
}

func (g *Game) PlayerOnTurn() int {
	return g.onturn
}

func (g *Game) SetPlayerOnTurn(onTurn int) {
	g.onturn = onTurn
}

func (g *Game) SetPointsFor(player, pts int) {
	g.players[player].points = pts
}

func (g *Game) Alphabet() *alphabet.Alphabet {
	return g.alph
}

func (g *Game) CurrentSpread() int {
	return g.PointsFor(g.onturn) - g.PointsFor((g.onturn+1)%2)
}
