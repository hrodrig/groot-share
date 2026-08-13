# Phase 6 Discussion Log

**Mode:** yolo lock from SPEC. User: "sigue... commits cortos."

| Topic | Choice | Locked |
|-------|--------|--------|
| Audit fields | actor, action, object id/key, ts, IP; no secrets | ✓ |
| Delete | DELETE API + HTML POST; retention also deletes | ✓ |
| Retention | either keep_last or max_age (union); defaults 20/90 | ✓ |
| vps-s3 | delete bucket home, not staging | ✓ |
| Staging leftovers | grace sweep as incident log | ✓ |
