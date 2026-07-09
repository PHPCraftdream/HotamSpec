"""Canon: §Domain — the single active-domain resolver (env → pin → alphabetical).

RULE (R-active-domain-pin-not-alphabetical): exactly ONE function,
``resolve_active_domain(domains_root)``, implements the resolution order
shared by every tool that must agree on which domain is active:

  1. ``HOTAM_SPEC_ACTIVE_DOMAIN`` env var (must name an existing
     ``domains/<name>/`` directory).
  2. ``domains/.active-domain`` pin file (committed, version-controlled;
     must name an existing ``domains/<name>/`` directory).
  3. First ``domains/<name>/`` directory alphabetically (last-resort
     deterministic fallback so a fresh repo is never "lost",
     R-agent-never-lost).

Returns the domain NAME (a ``str``) or ``None`` when ``domains/`` is absent
or contains no eligible directories. Callers compose their own paths from
the name (``domains_root / name / "graph.py"``, ``... / "docs" / "gen"``,
etc.), keeping this module pure-data: no filesystem mutation, no imports of
domain-specific code, stdlib-only.

WHY a single resolver (the bug this closes): before this module, three
copies of the env→pin→alphabetical walk lived in
``hotam_spec.graph._active_domain_graph_file``,
``gen_spec._select_active_domain_dir`` and
``apply_proposal._resolve_content_graph`` — plus the pin-file path constant
was triplicated (``_ACTIVE_DOMAIN_PIN_FILE``) with an explicit "mirrors …
exactly" comment in each copy. Editing one without synchronizing the other
two was a real source of silent divergence (observed: the original
apply_proposal ignored the env var entirely and always targeted the
alphabetically-first domain). Centralizing the walk here makes divergence
structurally impossible — there is only one implementation.

The pin file lives at ``domains/.active-domain`` (NOT under
``spec/.runtime/``, which is gitignored ephemera per
R-task-spawn-log-runtime) so the default is a deliberate, auditable,
committed decision, not a local-only override.
"""

from __future__ import annotations

import os
from pathlib import Path

#: Environment variable overriding the active domain (highest priority).
#: CI / test harnesses set this to target a domain without mutating the
#: filesystem (R-deterministic-generation).
ENV_VAR = "HOTAM_SPEC_ACTIVE_DOMAIN"

#: Pin file naming the default active domain when the env var is unset.
#: Lives at ``<repo>/domains/.active-domain`` — COMMITTED and version-
#: controlled (unlike ``spec/.runtime/``, which is gitignored ephemera) so
#: the default is a deliberate, auditable decision. All three historical
#: resolvers referenced this same path; it now lives in ONE place.
PIN_FILENAME = ".active-domain"


def _eligible_domain_dirs(domains_root: Path) -> list[Path]:
    """Return domains/<name>/ sub-directories, sorted alphabetically.

    Excludes names starting with ``_`` (private/scaffold dirs). Returns an
    empty list when ``domains_root`` is absent — the legitimate "no domain
    yet" state.
    """
    if not domains_root.exists():
        return []
    return sorted(
        d for d in domains_root.iterdir() if d.is_dir() and not d.name.startswith("_")
    )


def resolve_active_domain(domains_root: Path) -> str | None:
    """Return the NAME of the active domain, or ``None`` if none exists.

    RULE: resolution order is (1) ``HOTAM_SPEC_ACTIVE_DOMAIN`` env var
    (must name an existing ``domains/<name>/`` directory), (2)
    ``domains/.active-domain`` pin file (must name an existing
    ``domains/<name>/`` directory), (3) the first domain alphabetically.

    Returns the domain's directory NAME (e.g. ``"hotam-spec-self"``), NOT a
    path — callers compose ``domains_root / name / …`` themselves so this
    function stays pure and reusable across graph.py / gen_spec.py /
    apply_proposal.py which each need a different leaf (``graph.py``,
    ``docs/gen/``, the directory itself).

    Canon: §Domain — the single active-domain resolver entry point.

    WHY env-var first, pin file second: the env var lets CI / test harnesses
    override the domain without mutating the filesystem
    (R-deterministic-generation); the pin file is the committed, deliberate
    default for everyday use — with >= 2 domains present, "first
    alphabetically" is an accident of naming, not a decision (see the
    module docstring for the full historical WHY). Alphabetical stays as the
    last-resort fallback so a fresh repo with no pin file yet is never
    "lost" (R-agent-never-lost).
    """
    domain_dirs = _eligible_domain_dirs(domains_root)
    if not domain_dirs:
        return None
    names = {d.name for d in domain_dirs}

    env_domain = os.environ.get(ENV_VAR, "").strip()
    if env_domain and env_domain in names:
        return env_domain

    pin_file = domains_root / PIN_FILENAME
    if pin_file.exists():
        pinned = pin_file.read_text(encoding="utf-8").strip()
        if pinned and pinned in names:
            return pinned

    return domain_dirs[0].name


def pin_file_path(domains_root: Path) -> Path:
    """Return the canonical path to the ``domains/.active-domain`` pin file.

    Centralizes the pin-file path so every resolver agrees on its location
    (the historical triplication of ``_ACTIVE_DOMAIN_PIN_FILE`` with
    "mirrors … exactly" comments is replaced by this single helper).
    The file may or may not exist — callers guard with ``.exists()``.

    Canon: §Domain — the pin-file path accessor.
    """
    return domains_root / PIN_FILENAME
