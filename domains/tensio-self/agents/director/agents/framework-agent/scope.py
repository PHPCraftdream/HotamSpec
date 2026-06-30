"""Canon: §Agent — declares the scope of agent 'framework-agent'.

SCOPE is a tuple of R-id prefixes; the agent's CONSTITUTION block
filters g.requirements where id.startswith(any of these).

PURPOSE is a one-line machine-readable rationale used by AGENT-MAP.
"""

PURPOSE = (
    "Stewards the framework's structural invariants: atomicity audits, R↔check bijection, "
    "atom decomposition proposals. Operates only on spec/src/tensio/ + ALL_INVARIANTS; "
    "never edits spec/content/graph.py directly (returns ProposedRequirement JSON via "
    "apply_proposal queue)."
)

SCOPE = (
    "R-check-",
    "R-bijection-",
    "R-tool-",
    "R-atomicity-",
    "R-statemachine-",
    "R-conflict-",
    "R-decided-",
    "R-m-tag-",
    "R-typed-",
    "R-axis-",
    "R-anchor-",
    "R-speak-",
)
