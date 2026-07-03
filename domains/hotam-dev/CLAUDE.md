# hotam-dev

> This is a POINTER, not the live crystal for `hotam-dev`.
>
> The operator crystal (LIVE-STATE, CONSTITUTION, REPO-MAP, AGENT-MAP) does NOT
> live in this file. It materializes in the REPOSITORY-ROOT CLAUDE.md whenever
> `hotam-dev` is the active domain. gen_spec.py rebuilds that root crystal from
> whichever domain is pinned — it never populates per-domain CLAUDE.md files.
>
> To make `hotam-dev` the active domain and get its crystal:
>   echo hotam-dev > domains/.active-domain      # (or: export HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev)
>   cd spec && uv run python tools/gen_spec.py  # root CLAUDE.md becomes hotam-dev's crystal
> (create_domain.py --activate does both in one step.)

## Domain

модель разработки самого репозитория Hotam-Spec: волны, коммиты, спавны, верификационные ворота

## Goals

- волны садятся атомарно с зелёным T2 на границе
- каждый спавн и каждое применение оставляют трассу в .runtime-логах
- пуш только по явному слову стьюарда
