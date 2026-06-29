package app

// PreVerifierRegistry collects lock-free admission pre-verifiers contributed by
// modules. The app composes them and hands Verify to the mempool manager, which
// runs it before the admission mutex (mempool.type=app). First rejection wins; a
// nil result defers to the locked admission path.
type PreVerifierRegistry struct {
	verifiers []func([]byte) error
}

// Register adds a pre-verifier; nil is ignored.
func (r *PreVerifierRegistry) Register(v func([]byte) error) {
	if v != nil {
		r.verifiers = append(r.verifiers, v)
	}
}

// Verify runs the registered pre-verifiers, returning the first rejection or nil.
func (r *PreVerifierRegistry) Verify(raw []byte) error {
	for _, v := range r.verifiers {
		if err := v(raw); err != nil {
			return err
		}
	}
	return nil
}
