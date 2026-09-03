package catalog

import "strings"

// This file carries HelixLLM's model option set into HelixAgent's catalog so
// upper layers and final consumers see the same set the serving layer actually
// has running (FR-016), labelled with the host serving each one (FR-023), and
// able to tell a served model from a withheld one (FR-019).
//
// Everything here is the CONSUMER half of a wire contract
// (specs/002-adaptive-local-model-serving/contracts/model-listing.md). Two
// consequences follow, and both are deliberate:
//
//   - HelixAgent never DERIVES an identifier. Derivation belongs to the serving
//     layer, which owns the identity and the per-consumer rulesets; HelixAgent
//     carries the identifier it is given, verbatim, or carries nothing.
//   - HelixAgent never invents a fact the wire did not state. An option whose
//     availability was not reported is not "available"; a bare model id with no
//     identity does not acquire one here.

// Availability is what the serving layer reported about one option.
//
// It is a three-valued machine key rather than a bool because "the serving
// layer did not say" and "the serving layer said no" are different states with
// different meanings to a consumer, and a bool has nowhere to put the first.
// Only AvailabilityServing is usable; both other states are not (§11.4.6 — the
// absence of a serving claim is not a serving claim).
type Availability string

const (
	// AvailabilityUnreported is the zero value: nothing was reported about this
	// option's serving state. It is NEVER usable.
	AvailabilityUnreported Availability = ""
	// AvailabilityServing means the serving layer reports the option running.
	AvailabilityServing Availability = "serving"
	// AvailabilityWithheld means the option exists but is not being served;
	// WithheldReason says why, and the reason is what the user can act on.
	AvailabilityWithheld Availability = "withheld"
)

// Usable reports whether a consuming tool may present this option as one the
// user can select. Exactly one state qualifies (contract invariant 5: a model
// that is not actually being served is never listed as available).
func (a Availability) Usable() bool { return a == AvailabilityServing }

// WithheldReason is why one option was not offered.
//
// The values are NOT interchangeable: each implies a different remedy, and
// collapsing them into one generic "unavailable" destroys the only part of the
// answer a user can act on. The set is closed and the values are carried
// through unaltered. The user-facing wording is composed from these keys
// downstream, never stored here (CONST-046).
//
// The set has two halves. The first three describe why a host CANNOT RUN an
// option — the answer is about the option's demands against the machine. The
// last two describe why an option that could otherwise run IS NOT BEING SERVED
// — the answer is about the serving layer's own state, and they are not
// reducible to the first three (see ReasonProviderUnavailable).
//
// This set is one half of a wire contract; the serving layer states the same
// closed set on its side. Admitting a value here that no producer emits, or
// failing to admit one a producer does emit, breaks the contract in opposite
// directions — the second silently, because an unrecognised reason is discarded
// and the option arrives withheld with nothing to act on.
type WithheldReason string

const (
	// ReasonInsufficientResources: the host lacks the memory or storage the
	// option needs. A different host, or a smaller option, resolves it.
	ReasonInsufficientResources WithheldReason = "insufficient_resources"
	// ReasonUnsupportedConfiguration: nothing about this host's configuration
	// can run the option. More memory does not help.
	ReasonUnsupportedConfiguration WithheldReason = "unsupported_configuration"
	// ReasonExcludedByUsageTerms: the host could serve it, but the model's terms
	// forbid the declared usage. The remedy is never hardware.
	ReasonExcludedByUsageTerms WithheldReason = "excluded_by_usage_terms"

	// ReasonProviderUnavailable: the provider offering the option is not serving
	// right now — starting, loading, restarting, or unreachable. The option is
	// expected back and nothing about it has been withdrawn.
	//
	// This one is why the set could not stay at three. It is the distinction a
	// consuming tool needs in order to leave a restarting host's configuration
	// alone: mapped onto ReasonUnsupportedConfiguration it would say "more
	// memory does not help" about a backend that is simply still loading, which
	// is the opposite of the truth, and a tool acting on it would delete a
	// configuration that was about to become valid again.
	ReasonProviderUnavailable WithheldReason = "provider_unavailable"
	// ReasonIdentifierConflict: the identifier the serving layer derived for
	// this option is already bound to a DIFFERENT identity. Astronomically
	// unlikely, but it must surface as a withheld option rather than as an
	// option that silently replaced another one.
	ReasonIdentifierConflict WithheldReason = "identifier_conflict"
)

// Known reports whether r is one of the recorded reasons. The set is closed: a
// generic "unavailable" invented downstream is not a reason.
func (r WithheldReason) Known() bool {
	switch r {
	case ReasonInsufficientResources, ReasonUnsupportedConfiguration, ReasonExcludedByUsageTerms,
		ReasonProviderUnavailable, ReasonIdentifierConflict:
		return true
	default:
		return false
	}
}

// HelixLLMOption is one model option as the serving layer reports it.
//
// The split between ID and ModelIdentity is the whole point of the type. ID is
// the identifier — the charset-safe value derived by the serving layer for the
// consumer's rules as they stand. ModelIdentity is a VALUE: a label a user
// reads. It contains `/` and `:` and is rejected by consumer identifier guards,
// at least one of which is a shell-injection control. The two are carried in
// separate fields precisely so neither can be used in the other's place.
type HelixLLMOption struct {
	// ID is the derived, charset-safe identifier the serving layer produced.
	// HelixAgent carries it verbatim and never re-derives or widens it.
	ID string
	// ModelIdentity is the human-readable `helixllm/<host>/<model>[:<variant>]`
	// value. It identifies the option as HelixLLM-served and names its host
	// without anything else being consulted (FR-014).
	ModelIdentity string
	// Host is the machine serving the model (FR-023).
	Host string
	// OwnedBy is provenance as the serving layer reports it.
	OwnedBy string
	// Availability is the reported serving state.
	Availability Availability
	// WithheldReason is set only when Availability is AvailabilityWithheld.
	WithheldReason WithheldReason
}

// Offerable reports whether this option may be carried into the catalog at all.
//
// An option with an identifier the consumer's charset forbids is dropped rather
// than carried, because the only two alternatives are both forbidden: widening
// the consumer's guard (FR-014a — one of those guards prevents command
// injection), or substituting the human-readable identity for the identifier,
// which is the same widening by another route. Dropping is the honest outcome:
// the option is not offered, and nothing unsafe reaches a consumer's
// configuration.
func (o HelixLLMOption) Offerable() bool { return identifierSafe(o.ID) }

// identifierSafe applies the consuming tools' identifier rules AS THEY STAND —
// `^[A-Za-z][A-Za-z0-9_-]*$`, the intersection of the alias-name rule and the
// provider-id charset guard the Claude Toolkit enforces on a value it later
// interpolates and re-parses.
//
// This is an application of that rule at the propagation boundary, never a
// relaxation of it. If a future identifier fails this check the fix belongs in
// the serving layer's derivation, never here and never in the consumer's guard.
func identifierSafe(id string) bool {
	if id == "" {
		return false
	}
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9', r == '_', r == '-':
			if i == 0 {
				return false // must open with a letter
			}
		default:
			return false
		}
	}
	return true
}

// HelixLLMSource yields the model option set the serving layer is offering.
//
// It MAY be nil (HelixLLM not wired, or its listing unreachable): in that case
// the catalog carries no option entries rather than fabricating a set, exactly
// as VerifiedModelSource does for the verifier.
type HelixLLMSource interface {
	HelixLLMOptions() []HelixLLMOption
}

// helixLLMEntries projects the option set into catalog entries.
//
// Name keeps the established `helixllm/<id>` grammar so the entry sits in the
// same uniformly-named namespace as every other target; Model carries the
// derived identifier a consumer needs; ModelIdentity carries the value a user
// reads. Enabled is bound to Availability.Usable() so that no field on the
// entry says "usable" about a model that is not being served.
func helixLLMEntries(src HelixLLMSource) []Entry {
	if src == nil {
		return nil
	}
	options := src.HelixLLMOptions()
	entries := make([]Entry, 0, len(options))
	for _, o := range options {
		if !o.Offerable() {
			continue
		}
		reason := o.WithheldReason
		if o.Availability != AvailabilityWithheld {
			// A reason belongs to a withholding and to nothing else; carrying
			// one on a served option would state a withholding that did not
			// happen.
			reason = ""
		}
		entries = append(entries, Entry{
			Name:           NameHelixLLM + "/" + o.ID,
			Kind:           KindModel,
			Provider:       NameHelixLLM,
			Model:          o.ID,
			ModelIdentity:  strings.TrimSpace(o.ModelIdentity),
			Host:           strings.ToLower(strings.TrimSpace(o.Host)),
			OwnedBy:        strings.TrimSpace(o.OwnedBy),
			Availability:   o.Availability,
			WithheldReason: reason,
			Enabled:        o.Availability.Usable(),
		})
	}
	return entries
}
