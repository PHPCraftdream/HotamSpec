"""Canon: §Loop — CLI entry-point subpackage (§4 portability W3).

Thin wrappers that re-export each tool's ``main()`` so the framework can be
invoked as ``hotam-<tool>`` after ``pip install`` (P4: CLI access through
entry points). The tools/ directory remains the canonical implementation
and stays runnable as ``python tools/<tool>.py`` for self-hosting dev.

Each wrapper adds ``spec/tools`` to ``sys.path`` (via _bootstrap) so the
tool module and its sibling imports resolve, then calls the tool's own
``main()``. No logic is duplicated.
"""
