"""Canon: §Domain — director agent identity marker (P22.C: no separate crystal).

P22.C consolidation: the director is the SOLE operator of this domain and
operates directly through root CLAUDE.md (there is exactly one CLAUDE.md in
the whole repo). This scope.py exists only to satisfy
R-domain-declares-director / check_domain_director_exists, which requires a
discoverable director agent identity per manifest.py's DIRECTOR field. There
is no accompanying CLAUDE.md, tools/, agents/, or docs/ here — those would
recreate the multi-file indirection this consolidation removed. If a real
second agent is ever spawned via create_agent.py, it gets its own full
scaffold; the director itself does not need one while it is the only operator.
"""

PURPOSE = "Sole operator of the hotam-spec-self domain; operates directly via root CLAUDE.md."
SCOPE: tuple[str, ...] = ()
