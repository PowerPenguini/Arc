package workflow

import "fmt"

func ValidateStepDefinitions(defs []StepDef) error {
	if len(defs) == 0 {
		return fmt.Errorf("empty setup step definitions")
	}
	seenIDs := map[StepID]struct{}{}
	for i, def := range defs {
		if def.ID == "" {
			return fmt.Errorf("step %d has empty id", i)
		}
		if def.Label == "" {
			return fmt.Errorf("step %q has empty label", def.ID)
		}
		if _, ok := seenIDs[def.ID]; ok {
			return fmt.Errorf("duplicate step id: %q", def.ID)
		}
		seenIDs[def.ID] = struct{}{}
	}
	return nil
}
