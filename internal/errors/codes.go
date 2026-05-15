package errors

type ChainrailError struct {
	Code       string
	Message    string
	Suggestion string
	Cause      error
}

func (e *ChainrailError) Error() string {
	if e.Suggestion != "" {
		return e.Message + " (suggestion: " + e.Suggestion + ")"
	}
	return e.Message
}

func (e *ChainrailError) Unwrap() error { return e.Cause }

const (
	CodeNotGitRepo     = "NOT_GIT_REPO"
	CodeNoGhAuth       = "NO_GH_AUTH"
	CodeTrunkMissing   = "TRUNK_MISSING"
	CodeAlreadyInit    = "ALREADY_INITIALIZED"
	CodeDirtyWorktree  = "DIRTY_WORKTREE"
	CodeNotOnStack     = "NOT_ON_STACK"
	CodeSlugTaken      = "SLUG_TAKEN"
	CodeGhCallFailed   = "GH_CALL_FAILED"
	CodeGitCallFailed  = "GIT_CALL_FAILED"
	CodeSquashDetected = "SQUASH_DETECTED"
	CodeRebaseConflict = "REBASE_CONFLICT"
)
