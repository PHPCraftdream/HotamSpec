"""Canon: §Domain — manifest of domain 'hotam-spec-self'."""

ID = "hotam-spec-self"
DESCRIPTION = "The methodology modeling itself — Hotam-Spec about itself as a tension graph."
GOALS = (
    "burn down SETTLED-unenforced to zero",
    "atomize all compound check_*",
    "every CLAUDE.md section auto-generated from substrate",
)
DIRECTOR = "director"

# --- doc-reader bindings (R-doc-readers-declared-not-guessed) ---------------
# Explicit role -> Stakeholder.id binding for hotam_spec.doc_readers.
# Keys are the portable ROLE_* hints declared in
# spec/src/hotam_spec/doc_readers.py (ROLE_OPERATOR, ROLE_DOMAIN_STEWARD,
# ROLE_FRAMEWORK_MAINTAINER); values MUST be ids of Stakeholders declared in
# this domain's graph.py. Read at generation time via
# hotam_spec.graph.active_domain_doc_readers() — never guessed from a
# stakeholder id's substring.
DOC_READERS = {
    "operator": "ai-agent",
    "domain-steward": "domain-user",
    "framework-maintainer": "framework-author",
}
