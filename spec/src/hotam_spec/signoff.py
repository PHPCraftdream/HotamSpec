"""Canon: §Signoff — the frozen provenance record of a human steward decision.

RULE (R-trust-anchor-mechanism, R-decided-needs-human-signoff): every
steward-approved transition (Conflict -> DECIDED/HELD, Assumption ->
HOLDS/DEAD/IMPLEMENTS) MUST be auditable from the substrate. Before this
record, `decided_by` lived only in the gitignored proposal JSON and evaporated
the moment the writer landed: the graph showed a DECIDED conflict with a
`decided_by` string but no WHEN, no VERBATIM words, no provenance INSTRUMENT.
The Signoff payload closes that gap — it is NOT a new node type (it does not
deserve a lifecycle/steward/axis of its own), it is a frozen dataclass payload
attached to the node the human already governed, exactly the same rationale
that keeps Variant a payload on Conflict (anti-RDF: do not over-model as nodes
what is elaborated content on an existing node).

WHY `instrument` is an explicit SEAM (not a hidden assumption): the steward has
NOT yet decided (R-decided-by-verifiable-signature is OPEN) whether to bind
provenance to git commit authorship or a cryptographic signature. `instrument`
names HOW this signoff was captured today (`personal` = a human approved it and
we trust the writer; `DEL-<n>` = a recorded delegation file authorizes it) and
leaves room for `git` / `crypto` without reshaping the record. A future wave
that lands a verifiable signature changes the VALUE of instrument, not the
SHAPE of the record — the seam makes that a data edit, not a schema migration.

WHY `date` is the only timestamp in the whole ontology (and a deliberately
coarse one): this wave fixes provenance, not temporal modeling. A full
timestamp layer is a SEPARATE wave's work; here we need exactly enough to say
'WHEN did the steward sign' so R-trust-anchor-mechanism is auditable. ISO
YYYY-MM-DD is human-readable, deterministic as a written string, and coarse
enough that a signoff is stable within a working day.

WHY `chosen_variant` lives HERE (not as a Conflict field): when a HELD conflict
resolves to DECIDED, the steward picked one of the >=2 elaborated variants. The
chosen variant id travels in the signoff so the Variants tuple itself is NEVER
erased (the anti-relitigation cargo — implies/costs of the NON-chosen variants
— survives the decision). check_signoff_chosen_variant_resolves enforces that a
non-empty chosen_variant is the id of one of the conflict's variants.
"""

from __future__ import annotations

from dataclasses import dataclass

#: The admitted instrument values today. `personal` is the default (a human
#: approved it, trust the writer). `DEL-<n>` points at a delegation file under
#: delegations/ that authorizes the decider. `git`/`crypto` are reserved for
#: the future verifiable-signature wave (R-decided-by-verifiable-signature).
SIGNOFF_INSTRUMENTS: frozenset[str] = frozenset({"personal", "DEL"})


@dataclass(frozen=True)
class Signoff:
    """Canon: §Signoff — the frozen provenance record of a human decision.

    RULE: `decided_by` MUST be a non-empty Stakeholder id (the signoff is
    meaningless without a named human — R-decided-needs-human-signoff).
    `date` MUST be an ISO YYYY-MM-DD string (coarse, deterministic, auditable).
    `verbatim` is OPTIONAL — the steward's own words, carried verbatim so the
    rationale is not re-paraphrased by the writer. `instrument` names HOW the
    signoff was captured (the verifiable-signature seam); defaults to
    `personal`. `chosen_variant` is OPTIONAL and meaningful only for a Conflict
    that resolved from HELD: the V-id of the variant the steward picked.

    WHY frozen: mirrors Variant — a captured decision is immutable history; the
    graph builds frozen objects and the loader round-trips them through source
    text. A mutable signoff would let a later edit silently rewrite WHO
    approved WHAT and WHEN, which is exactly the provenance loss this record
    exists to prevent.
    """

    decided_by: str
    date: str
    verbatim: str = ""
    instrument: str = "personal"
    chosen_variant: str = ""
