package loader

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ProfileAtoms is the named manifest profile for a Fact/Holds self-executing-
// atoms domain: it bundles the four independent manifest.json flags such a
// domain needs into one declaration. Confirmed against the code (task P1-3):
//
//   - discipline:"full" — the methodology-discipline opt-in the scenario
//     gates co-require (ResolveDiscipline → ontology.Graph.Discipline).
//   - requirements_authority:"code" — LoadGraph's code-projection check
//     (loader.go) accepts a consumer domain only via
//     `self_hosting || requirements_authority code`; "code" is the consumer-
//     domain route, and it is what turns the applyToGraph
//     Requirement/Rejection lock on.
//   - self_executing_atoms:true — the single gate on Fact/Holds method
//     discovery (internal/invariants/fact_methods.go: checkFactMethodHasPhrase,
//     checkFactMethodAtomic, checkOneSubjectPerFact are all honest no-ops
//     unless g.SelfExecutingAtoms).
//   - gen_profile:"consumer" — the gen-spec output set an external business
//     consumer wants (ResolveGenProfile).
//
// NOT included: conformance.rule_cases. It is read only by WithCase-semantics
// consumers (internal/invariants/conformance_audit.go and the ontology
// ConformanceConfig validation); Fact/Holds method discovery never touches
// it, so the atoms profile deliberately omits it.
const ProfileAtoms = "atoms"

// manifestProfiles is the known-profile table: profile name → the JSON
// literal each bundled flag resolves to when the manifest does not set the
// flag explicitly. It is the ONE place profile expansion is defined; both
// consumption paths share it:
//
//   - the typed path (expandManifestProfile, called from LoadManifest);
//   - the direct-probe path (profileJSONDefault, called from the Resolve*
//     readers ResolveDiscipline / resolveRequirementsAuthorityCode /
//     ResolveGenProfile, which re-open manifest.json for one key each and so
//     never see LoadManifest's typed expansion).
var manifestProfiles = map[string]map[string]json.RawMessage{
	ProfileAtoms: {
		"discipline":             json.RawMessage(`"full"`),
		"requirements_authority": json.RawMessage(`"code"`),
		"self_executing_atoms":   json.RawMessage(`true`),
		"gen_profile":            json.RawMessage(`"consumer"`),
	},
}

// expandManifestProfile validates m.Profile against the known-profile table
// and materializes the profile's default flag values into m's typed fields —
// but ONLY for flags not EXPLICITLY set in the raw manifest bytes (an
// explicit flag always wins over the profile default). A manifest with no
// "profile" field is returned unchanged; an unknown profile name is an error.
// Called from LoadManifest after the decode, so every downstream consumer —
// typed or probe — sees the resolved flags.
func expandManifestProfile(m *DomainManifest, data []byte) error {
	if m.Profile == "" {
		return nil
	}
	defaults, known := manifestProfiles[m.Profile]
	if !known {
		return fmt.Errorf("unknown manifest profile %q (known profiles: %s)", m.Profile, knownProfileNames())
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		fields = nil // tolerate; data already decoded once in LoadManifest
	}
	apply := func(key string, dst any) {
		if _, explicit := fields[key]; explicit {
			return
		}
		if raw, ok := defaults[key]; ok {
			_ = json.Unmarshal(raw, dst)
		}
	}
	apply("discipline", &m.Discipline)
	apply("requirements_authority", &m.RequirementsAuthority)
	apply("gen_profile", &m.GenProfile)
	apply("self_executing_atoms", &m.SelfExecutingAtoms)
	return nil
}

// profileJSONDefault resolves ONE flag through the known-profile table for
// the direct-probe Resolve* readers. data is raw manifest.json bytes; the
// result is the profile's JSON literal for key, or false when there is no
// known profile or the key is explicitly set in the manifest (explicit wins,
// matching expandManifestProfile). Tolerant like every probe: malformed JSON
// is a soft no-op.
func profileJSONDefault(data []byte, key string) (string, bool) {
	var probe struct {
		Profile string `json:"profile"`
	}
	if json.Unmarshal(data, &probe) != nil || probe.Profile == "" {
		return "", false
	}
	defaults, known := manifestProfiles[probe.Profile]
	if !known {
		return "", false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return "", false
	}
	if _, explicit := fields[key]; explicit {
		return "", false
	}
	raw, ok := defaults[key]
	return string(raw), ok
}

// knownProfileNames returns the table keys in stable sorted order, for the
// unknown-profile error message.
func knownProfileNames() string {
	names := make([]string, 0, len(manifestProfiles))
	for name := range manifestProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
