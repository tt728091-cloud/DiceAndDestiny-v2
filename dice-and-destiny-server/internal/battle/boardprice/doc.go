// Package boardprice prices player ability boards: a die, a board template
// and the player's slot picks become abilities whose damage pays more the
// harder they are to roll, while every board averages the same per turn.
//
// Menus (LoadMenus) are the Board Builder's choices, all data:
// content/progression_v1/board_templates.yaml (pricing numbers, difficulty
// bands, dice, defenses, each family's flex cost menu and the templates) and
// content/progression_v1/riders.yaml (riders and the exchange rates every
// rider is priced at). Costs are written in die roles (content.DieRoles), so
// one template serves every symbol order of its family. Every menu entry
// carries an unlock rule; the first pass accepts only "open".
//
// The package must not import internal/battle/lookahead (the AI work).
package boardprice
