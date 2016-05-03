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

package main

import "space"

func main(){
	var play space.Playground

	play.InitPlayground()

	defer play.CanonicMode()
	play.RawMode()

	go play.Events()

	play.InitScreen()
	play.MoveSprite(space.DIR_LEFT)

	go play.DeployEnemies(space.STD_ENEMY_GROUP)

	for { play.ActionKey() }
}
