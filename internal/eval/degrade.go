package eval

import (
	"fmt"

	"github.com/fruitriin/EnumaElish/internal/dsl"
)

// approveCommand is the human-side command referenced in degrade hints.
// The hint deliberately contains the procedure only, never a token: the hint
// lands in the agent's context, and an embedded token would let the agent
// approve itself (Plan 0022 Phase 3 threat model).
const approveCommand = "ccchain approve --last"

// ResolveAsk resolves an ask result at the hook layer using the runtime
// permission mode (Plan 0022 Phase 2). In modes where an ask dialog cannot
// reach a human (auto, dontAsk, unknown), ask silently turns into a classifier
// decision or an unexplained rejection — neither is the "check with a human"
// the rule author asked for. So ask degrades explicitly:
//
//   - deny direction (default): deny + hint with the approval procedure,
//     turning the block into an asynchronous conversation
//   - allow direction (rule opt-in): warn, letting the call through while
//     landing the caution text in the agent's context
//
// Resolution order: strategy deny-all > rule `unattended:` >
// settings `ask_degrade_default` > built-in deny.
// Non-ask results pass through untouched.
func ResolveAsk(result *Result, permissionMode string, settings *dsl.Settings) *Result {
	if result == nil || result.Action != dsl.ActionAsk {
		return result
	}

	strategy := dsl.AskStrategyDegrade
	degradeDefault := dsl.ActionDeny
	if settings != nil {
		if settings.AskStrategy != "" {
			strategy = settings.AskStrategy
		}
		if settings.AskDegradeDefault != "" {
			degradeDefault = settings.AskDegradeDefault
		}
	}

	switch strategy {
	case dsl.AskStrategyPassthrough:
		return result

	case dsl.AskStrategyDenyAll:
		// Most conservative: every ask becomes deny+hint regardless of mode,
		// overriding any `unattended: allow`.
		return degradeToDeny(result, permissionMode)

	default: // dsl.AskStrategyDegrade
		if ClassifyMode(permissionMode) == ModeInteractive {
			return result
		}
		direction := result.Unattended
		if direction == "" {
			direction = degradeDefault
		}
		if direction == dsl.ActionAllow {
			return degradeToWarn(result, permissionMode)
		}
		return degradeToDeny(result, permissionMode)
	}
}

func degradeToDeny(result *Result, permissionMode string) *Result {
	out := *result
	out.Action = dsl.ActionDeny
	out.Message = joinMessage(result.Message, degradeDenyNotice(permissionMode))
	return &out
}

func degradeToWarn(result *Result, permissionMode string) *Result {
	out := *result
	out.Action = dsl.ActionWarn
	out.Message = joinMessage(result.Message, degradeAllowNotice(permissionMode))
	return &out
}

// degradeDenyNotice explains why the ask became a deny and how a human can
// approve it. Referenced template variables: {permission_mode} and
// {approve_command} equivalents are interpolated here directly.
func degradeDenyNotice(permissionMode string) string {
	if permissionMode == "" {
		permissionMode = "unknown"
	}
	return fmt.Sprintf(
		"ccchain: this command requires human approval, but the current mode (%s) cannot show a confirmation dialog. "+
			"To approve: re-run in an interactive session, or have the owner run `%s` in their own terminal, then retry the command.",
		sanitizeForMessage(permissionMode), approveCommand)
}

// degradeAllowNotice records that an ask rule was let through unattended.
func degradeAllowNotice(permissionMode string) string {
	if permissionMode == "" {
		permissionMode = "unknown"
	}
	return fmt.Sprintf(
		"ccchain: ask rule degraded to allow (unattended: allow) because the current mode (%s) cannot show a confirmation dialog. Proceeding with caution noted.",
		sanitizeForMessage(permissionMode))
}

func joinMessage(orig, notice string) string {
	if orig == "" {
		return notice
	}
	return orig + "\n" + notice
}
