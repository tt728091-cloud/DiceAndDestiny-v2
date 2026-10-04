package content

import "fmt"

func validateGeneralCardOperation(op BattleOperation) error {
	switch op.Modification {
	case "copy_die", "flip_die", "reroll_enemy_die", "reroll_defense_dice", "recover_discard", "dispel_positive", "save_threatened_card":
		if op.ExtraEnergy != 0 || op.BonusAmount != 0 {
			return fmt.Errorf("only boost_prevention accepts extra_energy and bonus_amount")
		}
	case "boost_prevention":
		if err := validateOperationAmount(op.Amount, false); err != nil {
			return err
		}
		if op.ExtraEnergy < 1 || op.BonusAmount < 1 {
			return fmt.Errorf("boost_prevention requires positive extra_energy and bonus_amount")
		}
	default:
		return fmt.Errorf("unknown general_card modification %q", op.Modification)
	}
	return nil
}
