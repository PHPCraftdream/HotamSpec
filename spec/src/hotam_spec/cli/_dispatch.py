"""Canon: §Loop — factory for single-tool CLI wrappers (§4 portability W3).

Before this module, ~24 of the 29 files under ``hotam_spec/cli/`` were
byte-for-byte copies of the same five-line template, differing only in the
tool module name they imported:

    from hotam_spec.cli._path_setup import ensure_tools_on_path
    ensure_tools_on_path()
    import <tool>  # noqa: E402
    def main() -> None:
        <tool>.main()

``make_main(tool_module_name)`` returns that ``main()`` function built from a
single shared implementation, so each per-tool file shrinks to one factory
call. Python packaging entry points (``[project.scripts]`` in pyproject.toml)
require a ``module:function`` target per command — that minimum one-file-per-
command structure cannot be collapsed further without either breaking `pip
install`'s entry-point mechanism or hand-writing a single mega-dispatcher
module whose per-command sections would just be this same factory call
repeated inline. One thin file per command, generated from one factory, is
the smallest structure that keeps every ``hotam-<tool>`` command independently
resolvable.

Two subcommand CLIs (``ticket.py``, ``delegation.py``) and the standalone
``land.py`` are NOT built from this factory — they dispatch to more than one
tool module based on ``sys.argv[1]`` and have their own bespoke ``main()``.
``mark_revisit.py`` uses the factory with an explicit ``import_as`` because
its tool module's filename (``mark_revisit_evaluated.py``) differs from the
CLI wrapper's short name.
"""

from __future__ import annotations

import importlib
import sys
from typing import Callable

from hotam_spec.cli._path_setup import ensure_tools_on_path


def make_main(tool_module_name: str, *, doc: str | None = None) -> Callable[[], None]:
    """Canon: §Loop — build a wrapper main() that delegates to tools/<tool_module_name>.py.

    ``tool_module_name`` is the bare module name as it lives in ``spec/tools/``
    (e.g. ``"gen_spec"`` for ``tools/gen_spec.py``). The returned function
    ensures ``spec/tools`` is on ``sys.path``, imports the tool module lazily
    (import happens on first call, not at wrapper-module import time — matches
    the previous per-file wrappers' behavior of importing at module load, but
    deferred here so ``_dispatch`` itself never needs the tool importable),
    and calls its ``main()``.

    ``doc`` optionally sets the returned function's ``__doc__`` (entry points
    don't need this, but it keeps ``--help``-adjacent introspection readable).
    """
    ensure_tools_on_path()

    def main() -> None:
        module = importlib.import_module(tool_module_name)
        rc = module.main()
        if rc:
            sys.exit(rc)

    if doc is not None:
        main.__doc__ = doc
    main.__name__ = "main"
    return main
