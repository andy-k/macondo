package board

// This file contains some sample filled boards, used solely for testing.

import "github.com/domino14/macondo/alphabet"

// VsWho is an enumeration
type VsWho uint8

const (
	// VsEd was a game I played against Ed, under club games 20150127vEd
	VsEd VsWho = iota
	// VsMatt was a game I played against Matt Graham, 2018 Lake George tourney
	VsMatt
	// VsJeremy was a game I played against Jeremy Hall, 2018-11 Manhattan tourney
	VsJeremy
	// VsOxy is a constructed game that has a gigantic play available.
	VsOxy
	// VsMatt2 at the 2018-11 Manhattan tourney
	VsMatt2
	// VsRoy at the 2011 California Open
	VsRoy
	// VsMacondo1 is poor_endgame_timing.gcg
	VsMacondo1
)

// SetToGame sets the board to a specific game in progress. It is used to
// generate test cases.
func (b *GameBoard) SetToGame(alph *alphabet.Alphabet, game VsWho) *TilesInPlay {
	// Set the board to a game
	switch game {
	case VsEd:
		// Quackle generates 219 total unique moves with a rack of AFGIIIS
		return b.SetFromPlaintext(`
cesar: Turn 8
   A B C D E F G H I J K L M N O   -> cesar                    AFGIIIS   182
   ------------------------------     ed                       ADEILNV   226
 1|=     '       =       '     E| --Tracking-----------------------------------
 2|  -       "       "       - N| ?AAAAACCDDDEEIIIIKLNOOOQRRRRSTTTTUVVZ  37
 3|    -       '   '       -   d|
 4|'     -       '       -     U|
 5|        G L O W S   -       R|
 6|  "       "     P E T     " E|
 7|    '       ' F A X I N G   R|
 8|=     '     J A Y   T E E M S|
 9|    B     B O Y '       N    |
10|  " L   D O E     "     U "  |
11|    A N E W         - P I    |
12|'   M O   L E U       O N   '|
13|    E H     '   '     H E    |
14|  -       "       "       -  |
15|=     '       =       '     =|
   ------------------------------
`, alph)
	case VsMatt:
		return b.SetFromPlaintext(`
cesar: Turn 10
   A B C D E F G H I J K L M N O      matt g                   AEEHIIL   341
   ------------------------------  -> cesar                    AABDELT   318
 1|=     '       Z E P   F     =| --Tracking-----------------------------------
 2|  F L U K Y       R   R   -  | AEEEGHIIIILMRUUWWY  18
 3|    -     E X   ' A   U -    |
 4|'   S C A R I E S T   I     '|
 5|        -         T O T      |
 6|  "       " G O   L O     "  |
 7|    '       O R ' E T A '    | ↓
 8|=     '     J A B S   b     =|
 9|    '     Q I   '     A '    | ↓
10|  "       I   N   "   N   "  | ↓
11|      R e S P O N D - D      | ↓
12|' H O E       V       O     '| ↓
13|  E N C O M I A '     N -    | ↓
14|  -       "   T   "       -  |
15|=     V E N G E D     '     =|
   ------------------------------
`, alph)
	case VsJeremy:
		return b.SetFromPlaintext(`
jeremy hall: Turn 13
   A B C D E F G H I J K L M N O   -> jeremy hall              DDESW??   299
   ------------------------------     cesar                    AHIILR    352
 1|=     '       N       '     M| --Tracking-----------------------------------
 2|  -       Z O O N "       A A| AHIILR  6
 3|    -       ' B '       - U N|
 4|'   S -       L       L A D Y|
 5|    T   -     E     Q I   I  |
 6|  " A     P O R N "     N O R|
 7|    B I C E '   A A   D A   E|
 8|=     '     G U V S   O P   F|
 9|    '       '   E T   L A   U|
10|  "       J       R   E   U T|
11|        V O T E   I - R   N E|
12|'     -   G   M I C K I E S '|
13|    -       F E ' T   T H E W|
14|  -       " O R   "   E   X I|
15|=     '     O Y       '     G|
   ------------------------------
`, alph)
	case VsOxy:
		// lol
		return b.SetFromPlaintext(`
cesar: Turn 11
   A B C D E F G H I J K L M N O      rubin                    ADDELOR   345
   ------------------------------  -> cesar                    OXPBAZE   129
 1|= P A C I F Y I N G   '     =| --Tracking-----------------------------------
 2|  I S     "       "       -  | ADDELORRRTVV  12
 3|Y E -       '   '       -    |
 4|' R E Q U A L I F I E D     '|
 5|H   L   -           -        |
 6|E D S     "       "       "  |
 7|N O '     T '   '       '    |
 8|= R A I N W A S H I N G     =|
 9|U M '     O '   '       '    |
10|T "   E   O       "       "  |
11|  W A K E n E R S   -        |
12|' O n E T I M E       -     '|
13|O O T     E ' B '       -    |
14|N -       "   U   "       -  |
15|= J A C U L A T I N G '     =|
   ------------------------------
`, alph)
	case VsMatt2:
		return b.SetFromPlaintext(`
cesar: Turn 8
   A B C D E F G H I J K L M N O   -> cesar                    EEILNT?   237
   ------------------------------     matt graham              EIJPSTW   171
 1|=     '       =       '     R| --Tracking-----------------------------------
 2|  -       "       "     Q - E| AABCDDDEEEEEHIIIIJLLLMNOPRSSSTTTUUVWWY  38
 3|    T I G E R   '     H I   I|
 4|'     -     O F       U     N|
 5|        O C E A N   P R A N K|
 6|  "       "   B A Z A R   "  |
 7|    '       '   '     A '    |
 8|=     '       M O O N Y     =|
 9|    '       D I F       '    |
10|  "       V E G   "       "  |
11|        -     S A n T O O R  |
12|'     -       '     O X     '|
13|    -       ' A G U E   -    |
14|  -       "       "       -  |
15|=     '       =       '     =|
   ------------------------------
`, alph)
	case VsRoy:
		return b.SetFromPlaintext(`
cesar: Turn 10
   A B C D E F G H I J K L M N O      roy                      WZ        427
   ------------------------------  -> cesar                    EFHIKOQ   331
 1|=     '       =     L U R I D| --Tracking-----------------------------------
 2|  - O     "       "       - I| WZ  2
 3|    U       '   P R I C E R S|
 4|O U T R A T E S       O     T|
 5|    V   -           - u     E|
 6|G " I   C O L O N I A L   " N|
 7|A   E S     '   '     T '   D|
 8|N     E       U P B Y E     E|
 9|J   ' R     M   ' O   R '   D|
10|A B   E N " A G A V E S   "  |
11|  L   N O   F   M I X        |
12|' I   A N   I '   D   -     '|
13|  G A T E W A Y s       -    |
14|  H   E   "       "       -  |
15|= T   '       =       '     =|
   ------------------------------
`, alph)
	case VsMacondo1:
		return b.SetFromPlaintext(`
teich: Turn 12
   A B C D E F G H I J K L M N O      cesar                    ENNR      379
   ------------------------------  -> teich                    APRS?     469
 1|J O Y E D     =       '     =| --Tracking-----------------------------------
 2|U - E L   V       "       -  | ENNR  4
 3|G   W O   I '   '       -    |
 4|A I   P   G   '       -     '|
 5|    F E T A         -        |
 6|  Y I R R S       C       "  |
 7|    L   I   O B I A     '    |
 8|U     H A I K A   L   '     =|
 9|N   Z   L   '   ' O F   '    |
10|I T E M I S E D   T O     "  |
11|T   B A N     E   T O        |
12|E   R U G     V   e D -     '|
13|  H A D     ' O '       -    |
14|W E       "   I N Q U E S T  |
15|E X E C       R       M O A N|
   ------------------------------
`, alph)
	}
	return nil
}