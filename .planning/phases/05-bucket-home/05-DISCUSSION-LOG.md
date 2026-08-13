# Phase 5 Discussion Log

**Mode:** yolo lock from SPEC + GFS-CONSENSUS (no questionnaire). User: "sigue".

| Topic | Choice | Locked |
|-------|--------|--------|
| Client | aws-sdk-go-v2 like groot; path-style when endpoint set | ✓ |
| HTTP keys | `{prefix}yyyy/mm/dd/{id}.tar.gz`; new id if exists | ✓ |
| Foreign keys | list whatever is under the prefix | ✓ |
| Copy fail | 201 transit + retry; not 5xx | ✓ |
| Tests | memory fake; no live bucket | ✓ |
| Give-up | deferred (SPEC open Q) | ✓ |
