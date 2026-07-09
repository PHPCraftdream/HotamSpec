"""Canon: §Loop — CLI entry-point subpackage (§4 portability W3).

Thin wrappers that re-export each tool's ``main()`` so the framework can be
invoked as ``hotam-<tool>`` after ``pip install`` (P4: CLI access through
entry points). The tools/ directory remains the canonical implementation
and stays runnable as ``python tools/<tool>.py`` for self-hosting dev.

Most wrappers are one-liners built from ``_dispatch.make_main(tool_name)``
(the shared factory: adds ``spec/tools`` to ``sys.path``, imports the tool
module, calls its ``main()`` — no logic duplicated across files). Python
packaging entry points require one ``module:function`` target per command,
so one file per command is the structural floor; the factory is what keeps
each of those files to a single call instead of a copy-pasted template.

Three wrappers are bespoke multi-subcommand dispatchers instead of factory
one-liners: ``ticket.py`` (create/list/show/move/edit/comment),
``delegation.py`` (delegate/record), and ``land.py`` (select/status/
verify-closure — see tools/land.py's docstring for why gate.py/gate_status.py/
closure.py were NOT physically merged into it).
"""
