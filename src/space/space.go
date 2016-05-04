// -----------------------------------------------------------------
// SystemInvaders - A tty game.
// Copyright (C) 2016  Gabriele Bonacini
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software Foundation,
// Inc., 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301  USA
// -----------------------------------------------------------------

// +build linux

package space

import (
         "fmt"
    	 "time"
	 "math/rand"
	 "os"
	 "os/exec"
	 "os/signal"
	 "syscall"
	 "unsafe"
	 "sync"
	 "strconv"
	 "strings"
)

const (
       MIN_ROWS        = 30
       MIN_COLS        = 70
       SPRITE_BEGIN    = 7
       SPRITE_END      = 4
       STD_BOSS_POINTS = 100
       STD_BOSS_DAMAGE = 12
       STD_ENEM_POINT  = 10
       STD_SHIELD_LEV  = 3
       STD_SHIELD_EXP  = -1
       STD_JUMP_LEN    = 10
       STD_ENEMY_GROUP = 10
       STD_DEST_REWARD = 10
       EN_MISS_SEQ_LEN = 4
       EN_MISS_ADJ     = 2
       RND_CORR        = 10
       RND_COL_ADJ     = 19
       RND_ROW_ADJ     = 15
       RND_ENEM_ADJ    = 25
       RND_ENEM_ADJ_SC = 9
       TIMER_LEVEL_AM  = 50000000
       TIMER_LEVEL_A   = 100000000
       TIMER_LEVEL_B   = 150000000
       TIMER_LEVEL_C   = 200000000
       TIMER_LEVEL_D   = 250000000
       NO_ERROR        = 0
       RUNTIME_ERROR   = 1
       DIMS_ERROR      = 2
       SCORE_OFFSET    = 9
       SCORE_TXT_OFFST = 14
       SCORE_IST_OFFST = 26
       SHIELD_OFFST    = 18
       INFO_OFFST      = 3
       MSG_OFFSET      = 9
       MAX_SCORE_LEN   = 8
       SPRITE_COLS     = 13
       SPRITE_ROWS     = 4
       SPRITE_COLS_GAP = 15
       INVASOR_COLS    = 5
       INVASOR_ROWS    = 3
       BOSS_COLS       = 8
       BOSS_ROWS       = 8
       DESTR_SEQUENCE  = 3
       DIR_LEFT        = -1
       DIR_RIGHT       = 1
       ROW_LOW_LIMIT   = 11
       ROW_START_LIMIT = 9
       COL_START_LIMIT = 6
       GET_TERMIOS     = syscall.TCGETS
       SET_TERMIOS     = syscall.TCSETS
       SOURCE_NAME     = "src/main/main.go"
       BINARY_NAME     = "a.out"
       COMPILER_NAME   = "go"
       RUN_PARAMETER   = "run"
       TERMINAL_DEV    = "/dev/tty"
       SPACE_CHARAC    = '\U00000020'
       BELL_SOUND      = '\U00000007'
)

type winsize struct {
    ws_row, ws_col                uint16
    ws_xpixel, ws_ypixel          uint16
}

type Termios struct {
	Iflag, Oflag, Cflag, 
	Lflag, Ispeed, Ospeed     uint32
	Cc                        [20]byte
}

type Playground struct{
	sync.Mutex

	wgConfirm                 sync.WaitGroup

	exploded,    critical     chan bool

	intSignal, winchSignal    chan os.Signal

	stop, start               bool

	termRow,       termCol,
	centTrmRow, centTrmCol, 
        curCol, score, shield     int 

	screen, sprite            [][]rune

	args                      []string

	oldTerm, newTerm          Termios
}

func (p *Playground) InitPlayground(){
	p.start       = false
	p.intSignal   = make(chan os.Signal, 1)
	p.winchSignal = make(chan os.Signal, 1)
	p.exploded    = make(chan bool, 1)
	p.critical    = make(chan bool, 1)

	signal.Notify(p.intSignal,   syscall.SIGINT)
	signal.Notify(p.winchSignal, syscall.SIGWINCH)

	rand.Seed(time.Now().UTC().UnixNano())
	p.getTermDims()

	p.centTrmRow = p.termRow / 2 
	p.centTrmCol = p.termCol / 2
        p.curCol       = p.termCol / 2
	p.score        = 0
	p.shield       = STD_SHIELD_LEV
	var spriteData = [SPRITE_ROWS][SPRITE_COLS]rune {
	    { '\U00000020','\U00000020','\U00000020','\U00000020','\U00002554','\U00002550','\U0000256C','\U00002550','\U00002557','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U00000020','\U00002560','\U00002550','\U00002569','\U00002550','\U00002563','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00002554','\U00002550','\U00002569','\U00002550','\U00002550','\U00002550','\U00002569','\U00002550','\U00002557','\U00000020','\U00000020'}, 
	    { '\U0000255A','\U00002550','\U00002569','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002569','\U00002550','\U0000255D'}}

	p.args = os.Args

	p.screen = make([][]rune, p.termRow)
	p.sprite = make([][]rune, SPRITE_ROWS)

	for i := range p.screen[:] {
		p.screen[i] = make([]rune, p.termCol)
	}

	for i := range spriteData[:] {
		p.sprite[i] = make([]rune, SPRITE_COLS)
		copy(p.sprite[i][:] , spriteData[i][:])
	}
}

func (p *Playground) InitScreen(){
	defer p.safeExitPanic("InitScreen")

	for i := range p.screen[:] {
		for j:= range p.screen[i]{
			p.screen[i][j] = '\U00000020'
		}
	}

	for i := range p.screen {
		fmt.Fprintf(os.Stderr, string(p.screen[i]))
	}

	var textRight = []rune {'\U00000053','\U00000048','\U00000049','\U00000045','\U0000004C','\U00000044','\U0000003A','\U00000020','\U00000033','\U00000020','\U00000053','\U00000043','\U0000004F','\U00000052', '\U00000045','\U0000003A','\U00000020','\U00000030'}

	copy(p.screen[p.termRow-1][(p.termCol - SCORE_IST_OFFST):] , textRight[:])
	
	var textLeft = []rune {'\U0000004D','\U0000006F','\U00000076','\U00000065','\U0000003A','\U00000020','\U00000061','\U0000002C','\U00000073','\U00000020','\U0000004A','\U00000075','\U0000006D','\U00000070','\U0000003A','\U00000020','\U0000007A','\U0000002C','\U00000078','\U00000020','\U00000046','\U00000069','\U00000072','\U00000065','\U0000003A','\U00000020','\U00000073','\U00000070','\U00000061','\U00000063','\U00000065','\U00000020','\U00000051','\U00000075','\U00000069','\U00000074','\U0000003A','\U00000020','\U00000071','\U00000020','\U00000020','\U0000004E','\U00000065','\U00000077','\U0000003A','\U00000020','\U00000072'}

	copy(p.screen[p.termRow-1][1:], textLeft[:])
}

func (p *Playground) getTermDims(){

        winSize := winsize{}
	_, _, execErr := syscall.Syscall(syscall.SYS_IOCTL,
			                 uintptr(0), uintptr(syscall.TIOCGWINSZ),
			                 uintptr(unsafe.Pointer(&winSize)))
	if execErr != 0 { panic("IOCTL Error") }

	p.termCol = int(winSize.ws_col)
	p.termRow = int(winSize.ws_row)

	if p.termRow < MIN_ROWS || p.termCol < MIN_COLS{
		p.CanonicMode()
		fmt.Fprintf(os.Stderr,"Terminal size too small: please increase the window dimensions\n")
		os.Exit(DIMS_ERROR)
	}
}

func (p *Playground) DeployEnemies(enemies int){
	for !p.stop{
		p.deployInvasors(enemies)
		p.deployBoss()
	}
}

func (p *Playground) deployEnemyMissile(col,row int, wg *sync.WaitGroup){
	defer p.safeExitPanic("deployEnemyMissile")

	var (
	    lines = [EN_MISS_SEQ_LEN]rune {'\U00000044', '\U0000002A', '\U0000002E', SPACE_CHARAC} 

	    coladj int = col + EN_MISS_ADJ
	    rowadj int = row + INFO_OFFST
	    end    int = p.termRow - SPRITE_END
	    begin  int = p.termRow - SPRITE_BEGIN - 1
	)

	for(rowadj < end && p.screen[rowadj + 1][coladj] == lines[3]){
		wg.Wait()
		if p.start { goto exit }
		rowadj++
		p.Lock()

		p.screen[rowadj][coladj]   = lines[0]
		p.screen[rowadj-1][coladj] = lines[3]

		p.refreshScreenUnlock(TIMER_LEVEL_B)
	}

	wg.Wait()

	if rowadj >= begin  && rowadj < end {
		p.changeShield(-1)
		if p.shield == STD_SHIELD_EXP { p.critical <- true }
	}

	fmt.Printf("%c",BELL_SOUND)
	for i:=1; i< EN_MISS_SEQ_LEN; i++{
		p.Lock() 
		p.screen[rowadj][coladj]     = lines[i]
		p.refreshScreenUnlock(TIMER_LEVEL_A)
	}

	exit:
}

func (p *Playground) changeShield(level int){
	if level < STD_SHIELD_LEV { 
		p.shield += level
	}else{
		p.shield = STD_SHIELD_LEV
	}
	thisShield := strconv.Itoa(p.shield)
	for i:=0; i<len(thisShield); i++{
		p.screen[p.termRow-1][p.termCol - SHIELD_OFFST + i] = rune (thisShield[i]) 
	}
}

func (p *Playground) changeScore(points int){
	p.score += points
	thisScore := strconv.Itoa(p.score)
	if( len(thisScore) <= MAX_SCORE_LEN){
		for i:=0; i<len(thisScore); i++{
			p.screen[p.termRow-1][p.termCol - (SCORE_OFFSET - i)] = rune (thisScore[i])
		}
	}
}

func (p *Playground) deployInvasors(enemies int){
	defer p.safeExitPanic("deployInvasors")

	var (
	    x int = p.centTrmCol
	    y int = 1

            //   ╔═╦═╗   *═*═*   *.*.*
            //   ╠═╬═╣   ╠*╬*╣   .*╬*.    *.* 
            //   ╝   ╚   * * *   * * *     *           

	    invasor = [INVASOR_ROWS][INVASOR_COLS]rune {
				 {'\U00002554','\U00002550','\U00002566','\U00002550','\U00002557'}, 
				 {'\U00002560','\U00002550','\U0000256B','\U00002550','\U00002563'}, 
				 {'\U0000255D','\U00000020','\U00000020','\U00000020','\U0000255A'}}

	    invasorDestr = [DESTR_SEQUENCE][INVASOR_ROWS][INVASOR_COLS]rune {
				 {{ '\U0000002A','\U00002550','\U0000002A','\U00002550','\U0000002A'},
				  { '\U00002560','\U0000002A','\U0000256B','\U0000002A','\U00002563'},
				  { '\U0000002A','\U00000020','\U0000002A','\U00000020','\U0000002A'}},
				 {{ '\U0000002A','\U0000002E','\U0000002A','\U0000002E','\U0000002A'}, 
				  { '\U0000002E','\U0000002A','\U0000256B','\U0000002A','\U0000002E'},
				  { '\U0000002A','\U00000020','\U0000002A','\U00000020','\U0000002A'}},
				 {{ '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'},
				  { '\U00000020','\U0000002A','\U0000002E','\U0000002A','\U00000020'},
				  { '\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020'}}} 

	    bline = []rune( "\U00000020\U00000020\U00000020\U00000020\U00000020") 

	    destroyed int = 0
	)

	var wg sync.WaitGroup
	for enemy := 0; enemy< enemies; enemy++  {
		var (
		    shoot1 int = rand.Intn(p.termRow - RND_ENEM_ADJ )
		    shoot2 int = shoot1 + RND_ENEM_ADJ_SC
		)

		for( p.screen[y+2][x] == p.screen[y+2][x+1]   && 
		     p.screen[y+2][x+2] == p.screen[y+2][x+3] &&
		     p.screen[y+2][x+3] == p.screen[y+2][x+4]){
				if p.start { goto exit }

				wg.Add(1)
				p.Lock()

				copy(p.screen[y-1][x:] , bline[:])
				for i:=0; i<3 ; i++{ 
					copy(p.screen[y+i][x:] , invasor[i][:]) 
				}
				p.refreshScreenUnlock(0)

				y++
				if( y == shoot1 || y == shoot2){ go p.deployEnemyMissile(x, y,  &wg)}
				wg.Done()

				if( y == (p.termRow - SPRITE_END) ){ break }

				time.Sleep(TIMER_LEVEL_C)
			}

			switch{
				case (y < (p.termRow - SPRITE_BEGIN - INFO_OFFST) ):
					<- p.exploded 
					destroyed++

					p.Lock()

					if( p.shield < STD_SHIELD_LEV && destroyed == STD_DEST_REWARD ){
						destroyed = 0
						p.changeShield(1)
					}
					p.Unlock()

					fmt.Printf("%c%c",BELL_SOUND, BELL_SOUND)
					copy(p.screen[y-1][x:] , bline[:])
					for h:= range invasorDestr[:]{
						p.Lock()
						for i:=0; i<3; i++{
							copy(p.screen[y+i][x:] , invasorDestr[h][i][:])
						}
						p.refreshScreenUnlock(TIMER_LEVEL_A)
					}

					p.Lock()
					for i:=0; i<3; i++{
						copy(p.screen[y+i][x:] , bline[:])
					}

					p.changeScore(STD_ENEM_POINT)
					p.refreshScreenUnlock(0)

				case ( y == (p.termRow - SPRITE_END) ):
					p.Lock()
					
					for i:=-1; i<2; i++{
						copy(p.screen[y+1][x:] , bline[:])
					}
					p.refreshScreenUnlock(0)

					p.critical <- true
				default:
					p.Lock()

					fmt.Printf("%c%c",BELL_SOUND, BELL_SOUND)
					copy(p.screen[y-1][x:] , bline[:])
					p.Unlock()

					for h:= range invasorDestr[:]{
						p.Lock()
						for i:=0; i<3; i++{
							copy(p.screen[y+i][x:] , invasorDestr[h][i][:])
						}
						p.refreshScreenUnlock(TIMER_LEVEL_A)
					}

					p.Lock()
					for i:=0; i<3; i++{
						copy(p.screen[y+i][x:] , bline[:])
					}
					p.refreshScreenUnlock(0)

					p.critical <- true 
			}

			y = 1
			x = RND_CORR + rand.Intn(p.termCol - RND_COL_ADJ)
	}
	exit:
}

func (p *Playground) deployBoss(){
	defer p.safeExitPanic("deployBoss")

	// ┏━━━━━━┓    *━*━━*━*   * *  * *
	// ┃彡ﾐ   ┃   ┃彡ﾐ   ┃    彡ﾐ        彡ﾐ     
	// ┃ᒡ◯ᵔ┃◯ᒡ┃   *ᒡ +┃ +*    * +  + *    *  *  
	// ┃┃  _┃ ┃   ┃┃  _┃ ┃     ┃   ┃     *   *  
	// ┃ \__/ ┃   * \__/ *    * \__/ *    * *   
	// ╚═╖══╖═╗   *═╖══╖═*    * ╖══╖ *    * *   
	//   ╬══╬       ╬══╬  
	//   ╝  ╚       *  *        ****  

	var (
	    boss = [BOSS_ROWS][BOSS_COLS]rune  {
	    {'\U0000256D','\U00000020','\U00002501','\U00002501','\U00002501','\U00002501','\U00000020','\U0000256E'},
	    {'\U00002503','\U00000020','\U0000256F','\U0000256F','\U00002570','\U00002570','\U00000020','\U00002503'},
	    {'\U00002503','\U000014A1','\U000025EF','\U00001D54','\U00002503','\U000025EF','\U000014A1','\U00002503'},
	    {'\U00002503','\U00002503','\U00000020','\U00000020','\U0000005F','\U00002503','\U00000020','\U00002503'},
	    {'\U00002503','\U00000020','\U0000005C','\U0000005F','\U0000005F','\U0000002F','\U00000020','\U00002503'},
	    {'\U0000255A','\U00002550','\U00002556','\U00002550','\U00002550','\U00002556','\U00002550','\U00002557'},
	    {'\U00000020','\U00000020','\U0000256C','\U00002550','\U00002550','\U0000256C','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U0000255D','\U00000020','\U00000020','\U0000255A','\U00000020','\U00000020'} }
   
	   bossDestruct = [DESTR_SEQUENCE][BOSS_ROWS][BOSS_COLS]rune  {
	   {{'\U0000002A','\U00002501','\U0000002A','\U00002501','\U00002501','\U0000002A','\U00002501','\U0000002A'},
	    {'\U00002503','\U00000020','\U0000256F','\U0000256F','\U00002570','\U00002570','\U00000020','\U00002503'},
	    {'\U0000002A','\U00000020','\U0000002B','\U00000020','\U00002503','\U0000002B','\U00000020','\U0000002A'},
	    {'\U00000020','\U00002503','\U00000020','\U00000020','\U0000002A','\U00002503','\U00000020','\U00000020'},
	    {'\U0000002A','\U00000020','\U0000005C','\U0000005F','\U0000005F','\U0000002F','\U00000020','\U0000002A'},
	    {'\U0000002A','\U00002550','\U00002556','\U00002550','\U00002550','\U00002556','\U00002550','\U0000002A'},
	    {'\U00000020','\U00000020','\U0000002A','\U00002550','\U00002550','\U0000002A','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020'}},
	   {{'\U0000002A','\U00000020','\U0000002A','\U00000020','\U00000020','\U0000002A','\U00000020','\U0000002A'},
	    {'\U00002503','\U00000020','\U0000256F','\U0000256F','\U00002570','\U00002570','\U00000020','\U00002503'},
	    {'\U0000002A','\U00000020','\U0000002B','\U00000020','\U00000020','\U0000002B','\U00000020','\U0000002A'},
	    {'\U00000020','\U00002503','\U00000020','\U00000020','\U00000020','\U00002503','\U00000020','\U00000020'},
	    {'\U0000002A','\U00000020','\U0000005C','\U0000005F','\U0000005F','\U0000002F','\U00000020','\U0000002A'},
	    {'\U0000002A','\U00000020','\U00002556','\U00002550','\U00002550','\U00002556','\U00000020','\U0000002A'}, 
	    {'\U00000020','\U00000020','\U0000002A','\U0000002A','\U0000002A','\U0000002A','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}},
	   {{'\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'},
	    {'\U00002503','\U00000020','\U0000256F','\U0000256F','\U00002570','\U00002570','\U00000020','\U00002503'},
	    {'\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020'},
	    {'\U00000020','\U0000002A','\U00000020','\U0000002A','\U00000020','\U0000002A','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U0000002A','\U00000020','\U0000002A','\U00000020','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'},
	    {'\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}}}
   
	   bline = [BOSS_COLS]rune {
	    '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'} 

	   x int = RND_CORR + rand.Intn(p.termCol - RND_COL_ADJ)
	   y int = 1
	   damage int = 0
	   shoot1 int = rand.Intn(p.termRow - RND_ROW_ADJ )
	   shoot2 int = shoot1 + RND_ENEM_ADJ_SC
        )

	var wg sync.WaitGroup
	for {
		if p.stop { goto exit}
		if(  p.screen[y+BOSS_ROWS][x]   != p.screen[y+BOSS_ROWS][x+1] || 
		     p.screen[y+BOSS_ROWS][x+2] != p.screen[y+BOSS_ROWS][x+3] ||
		     p.screen[y+BOSS_ROWS][x+3] != p.screen[y+BOSS_ROWS][x+4] ||
		     p.screen[y+BOSS_ROWS][x+5] != p.screen[y+BOSS_ROWS][x+6]){
				 damage++	
				 if (damage == STD_BOSS_DAMAGE || y >= (p.termRow - SPRITE_COLS_GAP)){ break }
		}

		wg.Add(1)
		p.Lock()

		copy(p.screen[y-1][x:] , bline[:])
		for i:=0; i<BOSS_ROWS; i++{
			copy(p.screen[y+i][x:] , boss[i][:])
		}
		p.refreshScreenUnlock(0)

		y++
		if( y == shoot1 || y == shoot2){
			go p.deployEnemyMissile(x+1, y+4, &wg)
			go p.deployEnemyMissile(x+2, y+4, &wg)
			go p.deployEnemyMissile(x+3, y+4, &wg)
		}

		wg.Done()
		if( y == (p.termRow - ROW_LOW_LIMIT) ){ break }

		time.Sleep(TIMER_LEVEL_C)

	}
	switch{
		case (y < (p.termRow - SPRITE_BEGIN - BOSS_ROWS) ):
			<- p.exploded 

			fmt.Printf("%c%c",BELL_SOUND, BELL_SOUND)
			for h:= range bossDestruct[:]{
				p.Lock()
				for i:=0; i<BOSS_ROWS; i++{
					copy(p.screen[y+i][x:] , bossDestruct[h][i][:])
				}
				p.refreshScreenUnlock(TIMER_LEVEL_A)
			}

			p.Lock()
			for i:=-1; i<BOSS_ROWS; i++{
				copy(p.screen[y+i][x:] , bline[:])
			}

			p.changeScore(STD_BOSS_POINTS)
			p.changeShield(STD_SHIELD_LEV)
			p.refreshScreenUnlock(0)

		case ( y == (p.termRow - ROW_LOW_LIMIT) ):
			p.Lock()
			for i:=-1; i<BOSS_ROWS; i++{
				copy(p.screen[y+i][x:] , bline[:])
			}
			p.refreshScreenUnlock(0)

			p.critical <- true
		default:
			fmt.Printf("%c%c",BELL_SOUND, BELL_SOUND)
			for h:= range bossDestruct[:]{
				p.Lock()
				for i:=0; i<BOSS_ROWS; i++{
					copy(p.screen[y+i][x:] , bossDestruct[h][i][:])
				}
				p.refreshScreenUnlock(TIMER_LEVEL_A)
			}

			p.Lock()
			for i:=-1; i<BOSS_ROWS; i++{
				copy(p.screen[y+i][x:] , bline[:])
			}
			p.refreshScreenUnlock(0)

			p.critical <- true 
	}
	exit: 
}

func (p *Playground) ActionKey(){
    defer p.safeExitPanic("ActionKey")

    var(
        missile  byte =  32  	// space : missile
        left     byte =  97  	// a: move left
        right    byte =  115 	// s: move right
        jleft    byte =  122  	// z: move left quick
        jright   byte =  120 	// x: move right quick
        quit     byte =  113 	// q: exit
        reset    byte =  114    // r: restart

        k []byte = make([]byte, 1)
    )

    os.Stdin.Read(k)
    switch k[0] {
	case left: 
		if !p.stop { p.MoveSprite(DIR_LEFT)}
	case right:
		if !p.stop { p.MoveSprite(DIR_RIGHT)}
	case jleft: 
		if !p.stop { for i:=0; i<STD_JUMP_LEN; i++{ p.MoveSprite(DIR_LEFT) }}
	case jright:
		if !p.stop { for i:=0; i<STD_JUMP_LEN; i++{ p.MoveSprite(DIR_RIGHT) }}
	case missile:
		if !p.stop { go p.deployMissile()}
	case reset:
		if !p.stop { p.restart(false) } else { p.wgConfirm.Done() }
	case quit: 
		p.safeExit() 
    }
    time.Sleep(TIMER_LEVEL_AM)
}

func (p *Playground) deployMissile(){
        defer p.safeExitPanic("deployMissile")

	var (
	    lines = []rune { '\U0000005E', '\U00002569', '\U0000002E', '\U0000002A' } 

	    coladj int = p.curCol + COL_START_LIMIT
	    rowadj int = p.termRow - ROW_LOW_LIMIT
	    startPos  int = p.termRow - ROW_START_LIMIT
	    safeRows  int = p.termRow - SPRITE_BEGIN
	)

	fmt.Printf("%c", BELL_SOUND)
	for(rowadj > 0 && p.screen[rowadj - 1][coladj] == SPACE_CHARAC ){
		if p.start { goto exit }
		rowadj--
		p.Lock()

		p.screen[rowadj][coladj]   = lines[0]
		p.screen[rowadj+1][coladj] = lines[1]
		if(rowadj < startPos){
			if(rowadj % 2 == 0){
					p.screen[rowadj+2][coladj] = lines[2]
					p.screen[rowadj+3][coladj] = lines[3]
			} else{
					p.screen[rowadj+2][coladj] = lines[3]
					p.screen[rowadj+3][coladj] = lines[2]
			}
			p.screen[rowadj+4][coladj] = SPACE_CHARAC
		}else{
			p.screen[rowadj+2][coladj] = SPACE_CHARAC
		}

		p.refreshScreenUnlock(0)

		time.Sleep(TIMER_LEVEL_A)
	}

	p.Lock() 

	for i:=0; i<5; i++{
		if(rowadj + i < safeRows ) {p.screen[rowadj + i][coladj] = SPACE_CHARAC}
	}

	p.refreshScreenUnlock(0)

	if(rowadj != 0){ p.exploded <- true }

	exit:
}

func (p *Playground) MoveSprite(direction int){
	defer p.safeExitPanic("MoveSprite")

	var end int
	switch direction {
		case DIR_LEFT:
			if p.curCol <= EN_MISS_ADJ { return }
			end = SPRITE_COLS
		case DIR_RIGHT:       
			if p.curCol >= (p.termCol - SPRITE_COLS_GAP) { return }
			end = -1
		default:
			return
	}
	p.curCol+=direction
	p.Lock()

	for i := range p.sprite[:] {
		for j:= range p.sprite[i]{
			p.screen[p.termRow-(SPRITE_BEGIN-i)][j+p.curCol] = p.sprite[i][j]
			p.screen[p.termRow-(SPRITE_BEGIN-i)][p.curCol+end] = SPACE_CHARAC
		}
	}

	p.refreshScreenUnlock(0)
}

func (p *Playground) Events(){
	defer p.safeExitPanic("Events")

	select {
		case <- p.critical:
			p.stop = true
			p.destroySprite()
			p.restart(true)
		case <- p.intSignal:
			p.safeExit()
		case <- p.winchSignal:
			p.restart(false)
	}
}

func (p *Playground) destroySprite(){
	defer p.safeExitPanic("destroySprite")

	var explosionSpriteData = [DESTR_SEQUENCE][SPRITE_ROWS][SPRITE_COLS]rune { 
           {{ '\U00000020','\U00000020','\U00000020','\U00000020','\U0000002A','\U00002550','\U0000007C','\U00002550','\U0000002A','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U00000020','\U0000002A','\U00002550','\U0000007C','\U00002550','\U0000002A','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U0000002A','\U00002550','\U0000005C','\U00002550','\U0000002A','\U00002550','\U0000002F','\U00002550','\U0000002A','\U00000020','\U00000020'}, 
	    { '\U0000002A','\U00002550','\U0000002A','\U00002550','\U00002550','\U0000002A','\U00002550','\U00002550','\U0000002A','\U00002550','\U0000002A','\U00002550','\U0000002A'}},
	   {{ '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U0000002A','\U00000020','\U0000002A','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U0000002A','\U00000020','\U0000002A','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U0000002A','\U0000005C','\U0000002A','\U0000002A','\U00000020','\U0000002F','\U00000020','\U0000002A','\U00000020','\U00000020'}, 
	    { '\U00000020','\U0000002A','\U00000020','\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020','\U0000002A','\U00000020','\U0000002A','\U00000020','\U0000002A'}},
	  {{ '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U0000002A','\U0000005C','\U0000002A','\U0000002A','\U00000020','\U0000002F','\U00000020','\U0000002A','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}, 
	    { '\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U0000002A','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020'}}}

	for h := range explosionSpriteData[:] {
		p.Lock()
		for i := range explosionSpriteData[h][:] {
			for j:= range explosionSpriteData[h][i]{
				p.screen[p.termRow-(SPRITE_BEGIN-i)][j+p.curCol] = explosionSpriteData[h][i][j]
			}
		}
		p.refreshScreenUnlock(TIMER_LEVEL_C)
	}
}

func (p *Playground) restart(confirm bool){
        defer p.safeExitPanic("restart")

	p.Lock()
	p.start = true
	p.stop  = true
	p.InitScreen()

	var (
		textScore = []rune {'\U00000053','\U00000043','\U0000004F','\U00000052','\U00000045','\U0000003A'}
	        textShield = []rune {'\U00000053','\U00000048','\U00000049','\U00000045','\U0000004C','\U00000044','\U0000003A'}
	        textAdvA = []rune {'\U00002554','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002557'}
	        textAdvB = []rune {'\U00002551','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00002551'}
	)
	copy(p.screen[p.termRow-1][(p.termCol - SCORE_TXT_OFFST):], textScore[:])
	copy(p.screen[p.termRow-1][(p.termCol - SCORE_IST_OFFST):], textShield[:])
	copy(p.screen[p.centTrmRow][(p.centTrmCol - MSG_OFFSET):], textAdvA[:])
	copy(p.screen[p.centTrmRow+1][(p.centTrmCol - MSG_OFFSET):], textAdvB[:])

	if confirm{
		var (
			textAdvCA = []rune {'\U00002551','\U00000020','\U00000047','\U00000041','\U0000004D','\U00000045','\U00000020','\U0000004F','\U00000056','\U00000045','\U00000052','\U00000020','\U00002551'}
			textAdvCB = []rune {'\U00002551','\U00000020','\U00000041','\U00000047','\U00000041','\U00000049','\U0000004E','\U00000020','\U0000003C','\U00000072','\U0000003E','\U00000020','\U00002551'}
		)
		copy(p.screen[p.centTrmRow+2][(p.centTrmCol - MSG_OFFSET):], textAdvCA[:])
		copy(p.screen[p.centTrmRow+3][(p.centTrmCol - MSG_OFFSET):], textAdvCB[:])

	}else{
		var (
			textAdvDA = []rune {'\U00002551','\U00000020','\U00000020','\U00000052','\U00000045','\U00000053','\U00000054','\U00000041','\U00000052','\U00000054','\U00000020','\U00000020','\U00002551'}
			textAdvDB = []rune {'\U00002551','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00002551'}
		)
		copy(p.screen[p.centTrmRow+2][(p.centTrmCol - MSG_OFFSET):], textAdvDA[:])
		copy(p.screen[p.centTrmRow+3][(p.centTrmCol - MSG_OFFSET):], textAdvDB[:])
	}

	var (
		textAdvE = []rune {'\U00002551','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00000020','\U00002551'}
		textAdvF = []rune {'\U0000255A','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U00002550','\U0000255D'}
	)
	copy(p.screen[p.centTrmRow+4][(p.centTrmCol - MSG_OFFSET):], textAdvE[:])
	copy(p.screen[p.centTrmRow+5][(p.centTrmCol - MSG_OFFSET):], textAdvF[:])

	p.changeScore(STD_ENEM_POINT)
	p.refreshScreenUnlock(0)

	if confirm{ 
		p.wgConfirm.Add(1) 
		p.wgConfirm.Wait() 
	}else {
		time.Sleep(TIMER_LEVEL_C)
	}

	env := os.Environ()   
	p.CanonicMode()
	if(strings.Contains(p.args[0], BINARY_NAME)){
		binary, lookErr := exec.LookPath(COMPILER_NAME)
		if lookErr != nil { panic(lookErr) }
		args := []string{COMPILER_NAME, RUN_PARAMETER, SOURCE_NAME}
		execErr := syscall.Exec(binary, args,  env)
		if execErr != nil { panic(execErr) }
	}else{
		binary, lookErr := exec.LookPath(p.args[0])
		if lookErr != nil { binary = p.args[0] }
		pars := []string{p.args[0]}
		execErr := syscall.Exec(binary, pars,  env)
		if execErr != nil { panic(execErr) }
	}
}

func (p *Playground) safeExitPanic(errMsg string){
	if e := recover(); e != nil {
		p.CanonicMode()
		fmt.Fprintf(os.Stderr, "Runtime Error: %s\n", errMsg)
		os.Exit(RUNTIME_ERROR)	
	}
}

func (p *Playground) safeExit(){
	p.CanonicMode()
	os.Exit(NO_ERROR)	
}

func (p *Playground) RawMode(){
        defer p.safeExitPanic("RawMode")

        p.newTerm.Iflag &^= (syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON)
        p.newTerm.Oflag &^= syscall.OPOST
        p.newTerm.Lflag &^= (syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN)
        p.newTerm.Cflag &^= (syscall.CSIZE | syscall.PARENB)
        p.newTerm.Cflag |= syscall.CS8

        p.newTerm.Cc[syscall.VWERASE]  = 1
        p.newTerm.Cc[syscall.VMIN]     = 1
        p.newTerm.Cc[syscall.VTIME]    = 0

        file, fileErr  := os.Open(TERMINAL_DEV)
	if fileErr != nil { panic(fileErr) }
        fd             := file.Fd()

        _, _, execErr := syscall.Syscall(syscall.SYS_IOCTL, fd, GET_TERMIOS, uintptr(unsafe.Pointer(&p.oldTerm)))
	if execErr != 0 { panic("IOCTL Error") }
        _, _, execErr = syscall.Syscall(syscall.SYS_IOCTL, fd, SET_TERMIOS, uintptr(unsafe.Pointer(&p.newTerm)))
	if execErr != 0 { panic(execErr) }

        fmt.Fprintf(os.Stderr,"%c[?%dl", 0x1B, 25)      // Disable cursor
}

func (p *Playground) CanonicMode(){
        fmt.Fprintf(os.Stderr,"%c[?%dh",   0x1B, 25)    // Enable cursor
        fmt.Fprintf(os.Stderr,"%c[%d;%dH", 0x1B, 1, 1)  // Tput 1,1
        fmt.Fprintf(os.Stderr,"%c[%dJ",    0x1B, 2)     // Clear

        file, fileErr  := os.Open(TERMINAL_DEV)
	if fileErr != nil { panic(fileErr) }
        fd             := file.Fd()

        _, _, execErr := syscall.Syscall(syscall.SYS_IOCTL, fd, SET_TERMIOS, uintptr(unsafe.Pointer(&p.oldTerm)))
	if execErr != 0 { panic("IOCTL Error") }
}


func (p *Playground) refreshScreenUnlock(timeWait time.Duration){
	defer p.safeExitPanic("refreshScreenUnlock")

	fmt.Fprintf(os.Stderr,"%c[%d;%dH", 0x1B, 1, 1) 
	for i := range p.screen {
		fmt.Fprintf(os.Stderr, string(p.screen[i]))
	}
	p.Unlock()
	if(timeWait > 0) {time.Sleep(timeWait)}
}
