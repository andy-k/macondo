package runner

import (
	"errors"
	"strings"

	"github.com/domino14/macondo/ai/player"
	"github.com/domino14/macondo/alphabet"
	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/cache"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/cross_set"
	"github.com/domino14/macondo/gaddag"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
	"github.com/domino14/macondo/movegen"
	"github.com/domino14/macondo/strategy"
)

// Basic game. Set racks, make moves

type GameRunner struct {
	game.Game
}

func NewGameRunner(conf *config.Config, opts *GameOptions, players []*pb.PlayerInfo) (*GameRunner, error) {
	opts.SetDefaults(conf)
	rules, err := game.NewBasicGameRules(
		conf, board.CrosswordGameBoard,
		opts.Lexicon.Distribution)
	if err != nil {
		return nil, err
	}
	return NewGameRunnerFromRules(opts, players, rules)
}

func NewGameRunnerFromRules(opts *GameOptions, players []*pb.PlayerInfo, rules *game.GameRules) (*GameRunner, error) {
	g, err := game.NewGame(rules, players)
	if err != nil {
		return nil, err
	}
	if opts.FirstIsAssigned {
		g.SetNextFirst(opts.GoesFirst)
	} else {
		// game determines it.
		g.SetNextFirst(-1)
	}
	g.StartGame()
	g.SetBackupMode(game.InteractiveGameplayMode)
	g.SetChallengeRule(opts.ChallengeRule)
	ret := &GameRunner{*g}
	return ret, nil
}

func (g *GameRunner) SetPlayerRack(playerid int, letters string) error {
	rack := alphabet.RackFromString(letters, g.Alphabet())
	return g.SetRackFor(playerid, rack)
}

func (g *GameRunner) SetCurrentRack(letters string) error {
	return g.SetPlayerRack(g.PlayerOnTurn(), letters)
}

func (g *GameRunner) NewPassMove(playerid int) (*move.Move, error) {
	rack := g.RackFor(playerid)
	m := move.NewPassMove(rack.TilesOn(), g.Alphabet())
	return m, nil
}

func (g *GameRunner) NewChallengeMove(playerid int) (*move.Move, error) {
	rack := g.RackFor(playerid)
	m := move.NewChallengeMove(rack.TilesOn(), g.Alphabet())
	return m, nil
}

func (g *GameRunner) NewExchangeMove(playerid int, letters string) (*move.Move, error) {
	alph := g.Alphabet()
	rack := g.RackFor(playerid)
	tiles, err := alphabet.ToMachineWord(letters, alph)
	if err != nil {
		return nil, err
	}
	leaveMW, err := game.Leave(rack.TilesOn(), tiles)
	if err != nil {
		return nil, err
	}
	m := move.NewExchangeMove(tiles, leaveMW, alph)
	return m, nil
}

func (g *GameRunner) NewPlacementMove(playerid int, coords string, word string) (*move.Move, error) {
	coords = strings.ToUpper(coords)
	rack := g.RackFor(playerid).String()
	return g.CreateAndScorePlacementMove(coords, word, rack)
}

func (g *GameRunner) MoveFromEvent(evt *pb.GameEvent) *move.Move {
	return game.MoveFromEvent(evt, g.Alphabet(), g.Board())
}

func (g *GameRunner) IsPlaying() bool {
	return g.Playing() == pb.PlayState_PLAYING
}

// Game with an AI player available for move generation.
type AIGameRunner struct {
	GameRunner

	aiplayer player.AIPlayer
	gen      movegen.MoveGenerator
}

func NewAIGameRunner(conf *config.Config, opts *GameOptions, players []*pb.PlayerInfo) (*AIGameRunner, error) {
	opts.SetDefaults(conf)
	rules, err := NewAIGameRules(
		conf, board.CrosswordGameBoard,
		opts.Lexicon.Name, opts.Lexicon.Distribution)
	if err != nil {
		return nil, err
	}
	g, err := NewGameRunnerFromRules(opts, players, rules)
	if err != nil {
		return nil, err
	}
	return addAIFields(g, conf)
}

func NewAIGameRunnerFromGame(g *game.Game, conf *config.Config) (*AIGameRunner, error) {
	gr := GameRunner{*g}
	return addAIFields(&gr, conf)
}

func addAIFields(g *GameRunner, conf *config.Config) (*AIGameRunner, error) {
	strategy, err := strategy.NewExhaustiveLeaveStrategy(
		g.LexiconName(),
		g.Alphabet(),
		conf,
		strategy.LeaveFilename,
		strategy.PEGAdjustmentFilename)
	if err != nil {
		return nil, err
	}

	gdObj, err := cache.Load(conf, "gaddag:"+g.LexiconName(), gaddag.CacheLoadFunc)
	if err != nil {
		return nil, err
	}
	gd, ok := gdObj.(*gaddag.SimpleGaddag)
	if !ok {
		return nil, errors.New("type-assertion failed; gaddag")
	}

	aiplayer := player.NewRawEquityPlayer(strategy)
	gen := movegen.NewGordonGenerator(gd, g.Board(), g.Bag().LetterDistribution())

	ret := &AIGameRunner{*g, aiplayer, gen}
	return ret, nil
}

func (g *AIGameRunner) MoveGenerator() movegen.MoveGenerator {
	return g.gen
}

func (g *AIGameRunner) GenerateMoves(numPlays int) []*move.Move {
	curRack := g.RackFor(g.PlayerOnTurn())
	oppRack := g.RackFor(g.NextPlayer())

	g.gen.GenAll(curRack, g.Bag().TilesRemaining() >= 7)

	plays := g.gen.Plays()

	// Assign equity to plays, and return the top ones.
	g.aiplayer.AssignEquity(plays, g.Board(), g.Bag(), oppRack)
	return g.aiplayer.TopPlays(plays, numPlays)
}

func (g *AIGameRunner) AssignEquity(plays []*move.Move, oppRack *alphabet.Rack) {
	g.aiplayer.AssignEquity(plays, g.Board(), g.Bag(), oppRack)
}

func (g *AIGameRunner) AIPlayer() player.AIPlayer {
	return g.aiplayer
}

func NewAIGameRules(cfg *config.Config, boardLayout []string,
	lexiconName string, letterDistributionName string) (*game.GameRules, error) {
	dist, err := cache.Load(cfg, "letterdist:"+letterDistributionName,
		alphabet.CacheLoadFunc)
	if err != nil {
		return nil, err
	}
	distLD, ok := dist.(*alphabet.LetterDistribution)
	if !ok {
		return nil, errors.New("type-assertion failed (letterDistribution)")
	}
	gdObj, err := cache.Load(cfg, "gaddag:"+lexiconName, gaddag.CacheLoadFunc)
	if err != nil {
		return nil, err
	}
	gd, ok := gdObj.(*gaddag.SimpleGaddag)
	if !ok {
		return nil, errors.New("type-assertion failed; gaddag")
	}
	board := board.MakeBoard(boardLayout)
	cset := cross_set.GaddagCrossSetGenerator{
		Gaddag: gd,
		Dist:   distLD,
	}
	lex := gaddag.Lexicon{gd}
	rules := game.NewGameRules(cfg, distLD, board, lex, cset)
	return rules, nil
}
